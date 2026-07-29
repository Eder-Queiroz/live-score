# Roteiro de implementação guiada

Este documento é o plano de trabalho do projeto, escrito no formato de estudo dirigido.

## Como usar

Cada tarefa tem quatro partes:

- **Conceito** — o que entender antes de escrever qualquer linha
- **Entregar** — o contrato, não a implementação
- **Armadilhas** — os erros que este passo específico costuma produzir
- **Aceite** — como saber que funcionou, de forma verificável

O ciclo de trabalho é: ler o conceito → tentar implementar → rodar o critério de aceite → pedir revisão. Não pule o aceite: um passo que "parece funcionar" sem verificação é dívida que aparece três fases depois.

**Regra do projeto:** todo arquivo executável é escrito manualmente. Dicas, contratos, explicações e revisão são bem-vindos; arquivos prontos não. O valor deste projeto está em poder defender cada decisão numa entrevista, e não se defende o que não se escreveu.

As fases 0 a 2 estão detalhadas porque são o trabalho imediato. As fases 3 a 7 estão em esboço e serão detalhadas quando alcançadas — detalhar antes seria especular sobre um sistema que ainda não existe.

---

## Fase 0 — Fundação

Objetivo: `make up` sobe a infraestrutura, `make test` roda verde, CI valida cada push. Nenhuma lógica de domínio ainda.

### 0.1 Esqueleto do repositório

**Conceito.** Layout de projeto Go. A distinção entre `cmd/`, `internal/` e `pkg/` não é estética: `internal/` é aplicada pelo compilador — nenhum módulo externo consegue importar de lá. Como este projeto não publica biblioteca, tudo que não é `main` vai em `internal/`, e `pkg/` não existe. Um binário por serviço em `cmd/<serviço>/main.go`.

**Entregar.** `go.mod`, `.gitignore`, a árvore de diretórios com um `main.go` mínimo por serviço, `Makefile` com `up`, `down`, `test`, `lint`, `build`.

**Armadilhas.**
- Nome do módulo. Use `github.com/Eder-Queiroz/live-score` desde o início — trocar depois obriga a reescrever todos os imports.
- `.gitignore` antes do primeiro commit, não depois. Binário compilado ou `.env` no histórico de um repositório público é difícil de remover de verdade.

**Aceite.** `go build ./...` compila. `git status` limpo depois de um build.

### 0.2 Stack local

**Conceito.** Kafka em modo KRaft dispensa o ZooKeeper — para um projeto novo em 2026 não há razão para usar ZooKeeper, e saber explicar isso já é um ponto. Entenda a diferença entre os listeners `PLAINTEXT` e `PLAINTEXT_HOST`: o primeiro é como os containers se enxergam entre si, o segundo é como sua máquina os alcança. Confundir os dois é a causa da maior parte dos "não consigo conectar no Kafka".

**Entregar.** `deploy/compose/docker-compose.yml` com Kafka (KRaft), Postgres e Redis. Healthchecks em todos. Volumes nomeados para persistência.

**Armadilhas.**
- `advertised.listeners` errado é o erro nº 1 do Kafka em Docker. O sintoma é enganoso: a conexão inicial funciona, o cliente recebe os metadados do broker, e então tenta conectar no endereço anunciado — que não existe da sua máquina. Falha depois de aparentemente conectar.
- Sem healthcheck, seus serviços sobem antes do Kafka estar pronto e falham no boot. Use `depends_on` com `condition: service_healthy`.
- Fixe versões das imagens. `latest` transforma seu ambiente em algo irreprodutível.

**Aceite.** `make up`, depois criar um tópico, produzir e consumir uma mensagem pelo CLI do Kafka de dentro do container. `psql` e `redis-cli` respondem.

### 0.3 Configuração e CI

**Conceito.** Configuração por variável de ambiente, carregada uma vez no boot para uma struct, validada na partida. Um serviço deve morrer imediatamente se a config estiver inválida, e não descobrir isso na primeira requisição.

**Entregar.** `internal/config` e um workflow do GitHub Actions com `go vet`, `golangci-lint`, `go test ./...` e build.

**Armadilhas.**
- Não leia `os.Getenv` espalhado pelo código. Fica impossível saber o que o serviço precisa para rodar.
- Nunca commite credencial do provider. `.env` no `.gitignore`, `.env.example` versionado.

**Aceite.** CI verde no primeiro push. Um serviço com variável obrigatória ausente falha no boot com mensagem clara.

### 0.4 Migrations

**Conceito.** Migrations versionadas e idempotentes, aplicadas por uma ferramenta e nunca por SQL manual. Decida agora quem aplica: um passo de deploy separado, ou o serviço no boot. Registre a escolha — é um ADR previsto.

**Entregar.** Migration inicial com as tabelas do design: dimensões, `matches`, `match_events` com `UNIQUE (match_id, event_key)` e índice em `(match_id, seq)`.

**Armadilhas.**
- Migration sem `down` te deixa preso quando algo der errado no meio.
- Pense no índice antes de haver dado. Criar índice em tabela grande em produção é operação com risco de lock.

**Aceite.** Aplicar da base vazia funciona. Reverter e reaplicar funciona. Aplicar duas vezes não é erro.

---

## Fase 1 — Caminho de escrita ponta a ponta

Objetivo: um evento sintético atravessa todo o pipeline e aparece no Postgres e no Redis, com `seq` correto. É a fase mais densa em conceito novo do projeto.

### 1.1 Schema de eventos e a interface `Provider`

**Conceito.** A interface é o ponto de extensão que sustenta o ADR-0002 e a fase futura de esports. Ela deve ser definida a partir do que o **consumidor** precisa, não do que a API do provider devolve — se a assinatura vazar o formato do football-data.org, o simulador vai ter que fingir ser football-data.org, e a abstração não serviu para nada.

**Entregar.** `internal/events` com as structs de evento e snapshot; `internal/provider` com a interface. Algo na direção de:

```go
type Provider interface {
    Name() string
    LiveMatches(ctx context.Context) ([]MatchSnapshot, error)
    Match(ctx context.Context, id string) (MatchSnapshot, error)
}
```

**Armadilhas.**
- Não coloque `*http.Response`, `ETag` nem nada de HTTP na interface. O simulador não tem HTTP.
- Modele `MatchSnapshot` com o que o diff precisa comparar, e nada além.
- Campos opcionais: em Go, `int` zero e "ausente" são indistinguíveis. Placar 0×0 é um estado real e frequente. Ponteiro ou tipo explícito de opcional, aqui.

**Aceite.** Compila com duas implementações declaradas, uma delas ainda vazia.

### 1.2 Simulador

**Conceito.** Este é o seu ambiente de teste determinístico e o gerador de carga da fase 5. Determinismo importa: com a mesma semente, a mesma sequência de eventos. Sem isso, um teste que falha não é reproduzível.

**Entregar.** `cmd/simulator` implementando `Provider`, gerando progressão realista de partida — kickoff, gols em minutos plausíveis, cartões, substituições, intervalo, fim. Taxa e semente configuráveis.

**Armadilhas.**
- Ele produz **snapshots**, não eventos. É tentador emitir eventos diretamente, mas isso pularia o diff engine e você perderia justamente o que quer testar.
- Inclua os casos difíceis desde já: placar decrescente por VAR, dois gols entre polls consecutivos, snapshot idêntico repetido. São eles que exercitam o diff.

**Aceite.** Rodar duas vezes com a mesma semente produz sequências idênticas de snapshots.

### 1.3 Diff engine

**Conceito.** O coração do sistema, e o trecho de código mais valioso do projeto inteiro. Função pura: `(anterior, novo) → []Event`. Sem I/O, sem rede, sem banco, sem relógio — se precisar de horário, receba como parâmetro, ou o teste vira não-determinístico.

Reserve tempo aqui. É onde mora a complexidade real, e é o que você vai explicar em qualquer entrevista sobre este projeto.

**Entregar.** `internal/diff`, com testes de tabela cobrindo: primeiro snapshot da partida (sem anterior), nenhuma mudança, gol simples, dois gols entre polls, cartão, substituição, mudança de período, **placar decrescente por VAR**, campos ausentes, snapshot fora de ordem, minuto retrocedendo.

**Armadilhas.**
- Dois gols entre polls consecutivos: o placar salta de 1×0 para 3×0. Você precisa emitir dois eventos `GOAL`, e provavelmente sem saber quem marcou o segundo. Decida o que fazer com informação parcial — e registre a decisão.
- Placar decrescente é `SCORE_CORRECTED`, nunca gol negativo. Log append-only não reescreve história.
- `event_key` precisa ser determinística **e** não colidir. Dois gols do mesmo jogador no mesmo minuto acontecem: o `ordinal` no hash existe exatamente para isso.
- Não confie no relógio do provider. Minuto pode retroceder, vir nulo, ou pular.

**Aceite.** Todos os casos de tabela passam. Os testes rodam em milissegundos e não usam Docker — se precisarem de container, a função não é pura e o desenho está errado.

### 1.4 Produtor Kafka

**Conceito.** Configuração de produtor é onde durabilidade se ganha ou se perde. `acks=all` significa que a escrita só é confirmada quando as réplicas mínimas a reconheceram. Produtor idempotente evita duplicata em retry interno. Entenda como o particionamento por chave funciona: mesma chave, mesma partição, ordem garantida — é isso que o ADR-0001 explora.

**Entregar.** `internal/kafka` com produtor configurado com `acks=all` e idempotência; `cmd/ingestor` publicando snapshots do simulador em `match.snapshots.v1`, com key = `match_id`.

**Armadilhas.**
- Producer sem `Flush`/`Close` no shutdown perde as mensagens em buffer. Aqui você perde dado de verdade, e é silencioso.
- Não crie um produtor por mensagem. Ele é caro, mantém conexões e deve viver o processo inteiro.
- Trate o erro de produção. Fire-and-forget contradiz o NFR-3 diretamente.
- Escolha a biblioteca cliente com critério e escreva o ADR previsto.

**Aceite.** Mensagens visíveis no tópico pelo CLI. Todos os eventos de um mesmo `match_id` na mesma partição — verifique, não assuma.

### 1.5 `event-derivation`

**Conceito.** Consumidor stateful. Ele precisa do snapshot anterior de cada partida para calcular o diff, e é a afinidade chave→partição que torna isso seguro. Entenda o consumer group: quem lê o quê, o que acontece num rebalance, e por que estado local sobrevive a isso apenas por causa do particionamento.

`seq` é monotônico por partida. Onde ele é armazenado, e o que acontece com ele num replay? Pense antes de codar — é a pergunta mais sutil da fase.

**Entregar.** `cmd/event-derivation` consumindo snapshots, mantendo o último por partida, chamando `internal/diff`, atribuindo `seq` e publicando em `match.events.v1`.

**Armadilhas.**
- Estado local se perde num restart. O primeiro snapshot depois de subir não tem anterior — decida: não emitir nada, ou reconstruir do Redis/Postgres. Ambos são defensáveis; escolha e justifique.
- Commit de offset antes de publicar o evento derivado perde eventos numa queda. Publique primeiro, comite depois.
- Num rebalance a partição pode migrar para outra instância que não tem o estado. É o mesmo problema do restart, e a mesma solução resolve.

**Aceite.** Simulador rodando, eventos derivados aparecem em `match.events.v1` com `seq` crescente e sem lacuna por partida.

### 1.6 `score-projector`

**Conceito.** Aqui a durabilidade se materializa. A ordem das operações é a decisão inteira: **persiste, depois comita o offset**. Invertido, uma queda entre as duas perde o evento permanentemente. Na ordem certa, uma queda causa reprocessamento — que é inofensivo, porque a constraint de unicidade absorve.

Isso é at-least-once com idempotência, e é deliberadamente escolhido em vez de perseguir exactly-once no transporte. Saber explicar essa escolha vale mais que implementá-la.

**Entregar.** `cmd/score-projector` consumindo eventos, com `INSERT` em `match_events`, upsert em `matches`, `HSET` em `match:{id}:state`, atualização de `match:{id}:tail` e `PUBLISH` em `match:{id}`. Commit manual de offset.

**Armadilhas.**
- Autocommit ligado é o bug silencioso desta fase. Desligue explicitamente.
- Conflito na constraint de unicidade **não é erro** — é o mecanismo funcionando. `ON CONFLICT DO NOTHING` e siga.
- Se o Redis falhar, o evento já está durável no Postgres. Não faça rollback do que é a verdade por causa de uma falha de cache — logue, meça e continue. Essa é a consequência prática do ADR-0005.
- Publique no pub/sub **depois** de escrever o estado. Invertido, um cliente pode receber a notificação e ler o estado antigo.

**Aceite.** Evento no Postgres e no Redis. `redis-cli SUBSCRIBE` mostra a publicação. Rodar o simulador duas vezes com a mesma semente **não** duplica linhas — este é o primeiro teste real de idempotência do projeto.

---

## Fase 2 — Caminho de leitura

Objetivo: FR-1 provado visualmente. É a fase que transforma o pipeline em produto.

### 2.1 API de leitura

**Conceito.** Este serviço não toca o Postgres para dado ao vivo. Leitura vem do Redis; Postgres só em cache miss, histórico e gap-fill.

**Entregar.** `cmd/web-service` com `GET /matches`, `GET /matches/{id}`, `/healthz`, `/readyz`.

**Armadilhas.**
- `healthz` e `readyz` são diferentes: vivo versus pronto para receber tráfego. Um `readyz` que só devolve 200 é decorativo.
- Cache miss precisa reconstruir do Postgres e reaquecer o Redis, não devolver 404.
- Sempre `context` com timeout nas chamadas a Redis e Postgres.

**Aceite.** Partida ao vivo servida sem query no Postgres — confirme pelo log de queries, não por dedução. Após `FLUSHALL` no Redis, a requisição ainda responde correto.

### 2.2 SSE

**Conceito.** A parte mais interessante de Go no projeto. Uma conexão SSE é uma resposta HTTP que nunca termina; você escreve e dá flush a cada evento. Milhares de conexões concorrentes são milhares de goroutines, e é por isso que Go se sustenta aqui — entenda o custo real de uma goroutine ociosa, é o número que fundamenta o NFR-1.

Estude também `context.Context` como sinal de desconexão do cliente: sem tratar isso, você acumula goroutines vazadas até a instância morrer.

**Entregar.** `GET /matches/{id}/stream` — headers de SSE, snapshot inicial na mesma conexão, assinatura do pub/sub do Redis, heartbeat a cada 15 s, gap-fill por `Last-Event-ID`, encerramento limpo ao desconectar.

**Armadilhas.**
- Sem flush explícito, a resposta fica em buffer e o cliente não recebe nada. É o primeiro bug que você vai encontrar.
- Sem snapshot inicial, tela vazia até o próximo gol — que pode não vir em 40 minutos.
- Sem heartbeat, proxy e load balancer matam a conexão silenciosamente. Só aparece depois do deploy.
- Vazamento de goroutine ao desconectar: sempre observe `r.Context().Done()`.
- Uma assinatura de Redis por conexão não escala. Multiplexe: uma assinatura por partida na instância, distribuindo internamente para as conexões interessadas.
- Gap-fill tem corrida: entre ler o histórico e assinar o canal, um evento pode escapar. Assine primeiro, faça buffer, depois preencha, depois drene o buffer.

**Aceite.** `curl -N` mostra o stream. Duas abas do navegador mostram o mesmo gol simultaneamente. Derrubar a rede por 2 minutos e reconectar resulta em timeline idêntica à de quem ficou conectado. Fechar 1000 conexões devolve o número de goroutines ao patamar inicial.

### 2.3 Client mínimo

**Conceito.** `EventSource` reconecta e envia `Last-Event-ID` automaticamente — o navegador faz metade do trabalho de resiliência de graça, desde que o servidor emita `id:` corretamente.

**Entregar.** `web/` — lista de partidas ao vivo, tela de partida com placar e timeline, atualizando via SSE. Sem framework, sem build.

**Armadilhas.**
- `EventSource` não aceita headers customizados. Se algum dia houver auth, ela vai por query param ou cookie.
- Reconexão automática só funciona se você emitir `id:` em cada evento.
- Sinalize o estado da conexão na tela: sem isso, você não distingue "sem gols" de "conexão morta".

**Aceite.** Simulador rodando, placar muda na tela sozinho. Grave o GIF agora — é a demonstração do README.

---

## Fases 3 a 7 — Esboço

Detalhamento quando alcançadas.

### Fase 3 — Provider real
Adapter do football-data.org. Token bucket no Redis. `ETag`/`If-None-Match`. Polling adaptativo por status. Leader election por advisory lock (ADR-0006). Classificação de erro e retry com backoff. Fixtures gravadas do provider real como golden files do diff.

**Prova:** jogo real na tela, e zero 429 em 24 h de operação contínua com duas réplicas ativas.

### Fase 4 — Estatísticas e histórico
`stats-projector`. Endpoints de time e jogador. Paginação por cursor. `ETag` e `Cache-Control` em dado imutável. Confirmar o que o plano gratuito realmente entrega de estatística de jogador (risco R2 do PRD) e ajustar o escopo do FR-2 conforme o achado.

### Fase 5 — Prova dos NFRs
OpenTelemetry no caminho completo, com a SLI de end-to-end lag decomposta por etapa. Prometheus e Grafana. **Teste de caos em CI** — matar consumidor e broker sob carga, assertar contagem exata sem duplicata. k6 subindo conexões SSE até o teto da máquina. Relatório em `docs/load-test.md` com número medido, gargalo identificado e extrapolação declarada como extrapolação.

Esta é a fase que converte os NFRs de afirmação em evidência. É a de maior retorno de portfólio e a que mais se corta por pressa — não corte.

### Fase 6 — Slice AWS
Terraform: remote state em S3 com lock em DynamoDB. OIDC no GitHub Actions, sem access key de longa duração. Ingestor em Lambda arm64 com EventBridge Scheduler. `web-service` em Fargate atrás de ALB (ADR-0004). Budget alarm. Tabela de custo real e `terraform destroy` no runbook.

### Fase 7 — Apresentação
README com diagrama e GIF. ADRs finalizados. Runbook. Relatório de carga. A seção de exclusões com justificativa — que é o que separa "usei muita tecnologia" de "escolhi estas e sei por quê".

---

## Ordem de corte

Se o tempo apertar, corte pelo fim, nunca pelo meio. As fases 0 a 2 já são um portfólio defensável: pipeline Kafka com garantias explícitas, tempo real funcionando, decisões documentadas.

A única exceção: **a fase 5 não deve ser cortada antes da 6**. Provar as garantias que você afirmou vale mais que ter infraestrutura na nuvem.
