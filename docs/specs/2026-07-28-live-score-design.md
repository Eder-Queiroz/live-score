# Design Técnico — Live Score

**Status:** aprovado · **Data:** 2026-07-28 · **PRD:** [`docs/PRD.md`](../PRD.md)

---

## 1. O princípio que organiza o sistema

Os requisitos não funcionais declarados no PRD se contradizem parcialmente. Escala de leitura para milhões de usuários empurra o desenho para cache agressivo, réplicas e fanout eventualmente consistente. Durabilidade e consistência forte empurram para escrita síncrona, fonte da verdade única e coordenação.

A contradição se dissolve quando se percebe que as duas exigências **não se aplicam à mesma coisa**. O sistema tem dois planos, com garantias declaradamente diferentes:

| | Caminho de escrita (ingestão) | Caminho de leitura (fanout) |
|---|---|---|
| **Garantia** | Durabilidade e ordem por partida | Eventualmente consistente |
| **Meta** | Zero eventos perdidos | p95 < 2 s até a tela |
| **Volume** | Dezenas de eventos/s | Milhões de leitores |
| **Escala com** | Número de partidas no mundo | Audiência |
| **Mecanismo** | Kafka `acks=all`, idempotência, Postgres como verdade | Redis, SSE, cache local |

Ninguém precisa de consistência forte para *ver* um placar — dois segundos de atraso são invisíveis. Precisa-se de consistência forte para **nunca perder um gol no log**. São problemas diferentes, em camadas diferentes, e recebem soluções diferentes.

A assimetria que viabiliza isso é uma propriedade do domínio: **a escrita é limitada pela realidade do futebol, a leitura pela audiência**. Uma partida gera de 20 a 50 eventos em 90 minutos. Mesmo com centenas de partidas simultâneas, a escrita é da ordem de dezenas por segundo — volume que um Postgres modesto absorve sem esforço. Já a leitura são milhões de pessoas querendo o mesmo placar. O caminho de escrita, portanto, pode ser conservador e caro por evento sem custo de escala, porque **não cresce com usuários**.

Detalhamento em [ADR-0005](../adr/0005-postgres-fonte-da-verdade-redis-como-cache.md).

## 2. Arquitetura

```
  football-data.org ──┐
                      ├──► ingestor ──────► Kafka: match.snapshots.v1
  match-simulator ────┘   (ativo-único,       (key = match_id)
                           polling                    │
                           adaptativo)                ▼
                                            event-derivation
                                             (diff de snapshots)
                                                      │
                                            Kafka: match.events.v1
                                              (key = match_id)
                                                      │
                             ┌────────────────────────┴───────────────┐
                             ▼                                        ▼
                      score-projector                          stats-projector
                             │                                        │
              ┌──────────────┼──────────────┐                         ▼
              ▼              ▼              ▼                    Postgres
        Postgres        Redis HASH    Redis PUBLISH          (agregados de time
      (log append-      (estado        match:{id}             e de jogador)
        only, verdade)  materializado)      │
                                            ▼
                             web-service (stateless, N réplicas)
                               GET /matches/{id}
                               GET /matches/{id}/stream  ◄── SSE
                               GET /teams/{id}/matches
                                            │
                                            ▼
                                      client mínimo
```

### 2.1 Serviços

| Serviço | Estado | Responsabilidade |
|---|---|---|
| `ingestor` | Ativo-único (leader election) | Poll do provider, respeito ao rate limit, publicação do snapshot bruto |
| `event-derivation` | Stateful por partição | Diff entre snapshots consecutivos, derivação de eventos, atribuição de `seq` |
| `score-projector` | Stateless | Persistência do log, materialização do estado no Redis, publicação no pub/sub |
| `stats-projector` | Stateless | Agregados de time e jogador |
| `web-service` | Stateless | API HTTP e SSE |
| `simulator` | — | Segunda implementação de `Provider`, para carga e demo determinística |

Consumer groups separados para cada projector: falha ou lentidão nas estatísticas não pode atrasar o placar ao vivo. Este é o motivo concreto de o Kafka estar aqui e não uma fila simples — múltiplos consumidores independentes lendo o mesmo fluxo, cada um no seu ritmo, cada um capaz de reprocessar do início sem afetar os outros.

## 3. As decisões centrais

### 3.1 Partition key = `match_id`

Kafka garante ordem apenas *dentro* de uma partição. A única ordem que importa neste domínio é a ordem dos eventos de uma mesma partida — não há relação causal entre um gol no Maracanã e um cartão em Old Trafford, e ninguém se importa com a ordem relativa entre eles.

Particionar por `match_id` entrega exatamente a garantia necessária, e nada além:

- Todos os eventos de uma partida caem na mesma partição, logo são ordenados
- Uma partida sempre vai para o mesmo consumidor, o que permite que `event-derivation` seja stateful com segurança
- Partidas distintas paralelizam livremente entre partições

Custo aceito: uma partida de audiência gigantesca gera partição mais quente. Como o volume de escrita por partida é minúsculo em termos absolutos, isso não é um problema de throughput.

Detalhamento em [ADR-0001](../adr/0001-particionamento-por-match-id.md).

### 3.2 A ingestão é um diff engine, não um receptor de eventos

O provider responde *"a partida está 2×1 aos 67 minutos"*. Não responde *"aconteceu um gol"*. A diferença é estrutural e determina o desenho da ingestão: alguém precisa comparar o snapshot atual com o anterior e **derivar** os eventos.

Esse diff mora num serviço separado, consumindo o tópico de snapshots brutos, e não dentro do `ingestor`. O motivo é a capacidade de replay.

O diff é a parte do sistema com maior probabilidade de conter bug, porque é onde o mundo real é sujo: VAR anula gol, provider corrige placar retroativamente, ordem de campos muda, dado chega incompleto. Quando um desses bugs for descoberto — e será — a correção precisa poder ser aplicada **ao passado**. Com os snapshots brutos preservados no Kafka, corrigir significa: ajustar o código, resetar o offset do consumer group, reprocessar. Se o diff estivesse embutido no `ingestor`, os snapshots nunca teriam sido persistidos, e o dado bruto estaria perdido para sempre.

Consequência de projeto: `internal/diff` é uma **função pura** — recebe `(anterior, novo)` e devolve `[]Event`, sem I/O, sem rede, sem banco. A parte mais complexa do sistema passa a ser a mais fácil de testar.

Detalhamento em [ADR-0002](../adr/0002-diff-engine-como-servico-separado.md).

### 3.3 Rate limit é estado compartilhado

10 requisições por minuto é o limite **da chave de API**, não da instância. Duas réplicas do `ingestor` com contador local cada uma somam 20 req/min, e o resultado é 429 seguido de bloqueio.

Três mecanismos combinados mantêm o consumo dentro do orçamento:

1. **Token bucket no Redis** — orçamento compartilhado entre todas as réplicas
2. **Requisições condicionais** (`ETag` / `If-None-Match`) — um 304 não gasta cota em vários providers, e a maior parte dos polls de uma partida parada retorna estado idêntico
3. **Polling adaptativo** — partida ao vivo a cada 15 s; agendada a cada 10 min; encerrada não é consultada

O terceiro item é o que faz o orçamento fechar: sem priorização, 10 req/min divididas entre todas as partidas do dia dariam uma atualização a cada vários minutos por partida. Concentrando a cota nas partidas ao vivo, o intervalo efetivo cai para segundos onde importa.

## 4. Modelo de dados

### 4.1 Postgres — fonte da verdade

Duas categorias: **dimensões**, que vêm do provider e mudam raramente, e o **log**, que é append-only e nunca sofre `UPDATE`.

```
competitions   (id, external_id, provider, name, country, season)
teams          (id, external_id, provider, name, short_name, crest_url)
players        (id, external_id, provider, name, position, team_id)

matches        (id, external_id, provider, competition_id,
                home_team_id, away_team_id, kickoff_at,
                status, home_score, away_score,
                last_seq, updated_at)

match_events   (id, match_id, seq, event_key, type,
                period, minute, team_id, player_id,
                score_home, score_away,
                payload jsonb, occurred_at, ingested_at)
                UNIQUE (match_id, event_key)
                INDEX  (match_id, seq)

team_match_stats    (match_id, team_id, ...)
player_match_stats  (match_id, player_id, ...)
```

`matches` é uma **projeção** de `match_events`, materializada no próprio banco para evitar recalcular o placar em toda leitura fria. Se divergir, é reconstruível a partir do log. O log é a verdade; tudo mais é derivado.

Snapshots brutos **não** vão para o Postgres. Vivem no Kafka com retenção de 7 dias, suficiente para replay do diff, e evitam uma tabela volumosa que nenhuma consulta de produto leria.

Agregados de temporada são **calculados na leitura**, com índice adequado — não materializados. Materialized view entra se e quando a medição mostrar necessidade.

### 4.2 `seq` e `event_key` resolvem problemas distintos

- **`seq`** — contador monotônico por partida, atribuído pelo `event-derivation`. Serve para **ordenação** e para o `Last-Event-ID` do SSE.
- **`event_key`** — fingerprint determinística do conteúdo: `hash(match_id, type, period, minute, player_id, ordinal)`. Serve para **deduplicação**.

O `seq` sozinho não serve como chave de dedup. Num replay, o contador pode reiniciar ou desalinhar, e o mesmo gol seria inserido novamente com `seq` diferente. A fingerprint deriva do conteúdo do evento, então reprocessar o mesmo snapshot mil vezes produz mil vezes a mesma chave, e o `UNIQUE` absorve as repetições silenciosamente.

É esse par que torna o replay seguro. E replay seguro é o que permite tratar o Kafka como mecanismo de reprocessamento, e não apenas como canal de transporte.

### 4.3 Schema dos eventos

```json
{
  "event_id": "01J8...",
  "event_key": "sha256:...",
  "match_id": "...",
  "seq": 42,
  "type": "GOAL",
  "period": "SECOND_HALF",
  "minute": 67,
  "team_id": "...",
  "player_id": "...",
  "score": { "home": 2, "away": 1 },
  "occurred_at": "2026-07-28T20:07:31Z",
  "ingested_at": "2026-07-28T20:07:33Z",
  "source": { "provider": "football-data", "snapshot_hash": "..." },
  "payload": {}
}
```

Tipos no MVP: `MATCH_STATUS_CHANGED`, `PERIOD_CHANGED`, `GOAL`, `CARD`, `SUBSTITUTION`, `SCORE_CORRECTED`.

O último merece atenção. Quando o VAR anula um gol, o provider simplesmente devolve um placar menor no snapshot seguinte. O diff engine detecta placar **decrescente** e emite `SCORE_CORRECTED`, em vez de fabricar um gol negativo. Um log append-only não reescreve história — registra a correção, exatamente como um estorno contábil. A timeline resultante mostra o gol e mostra a anulação, que é o que de fato aconteceu.

Sem Schema Registry no MVP: structs Go compartilhadas em `internal/events`, versionamento no nome do tópico. Justificativa e gatilho de reversão em [ADR-0003](../adr/0003-sem-schema-registry.md).

### 4.4 Redis — layout de chaves

| Chave | Tipo | Papel | TTL |
|---|---|---|---|
| `match:{id}:state` | HASH | placar, minuto, período, status, `last_seq` | 6 h após o fim |
| `match:{id}:tail` | LIST (cap ~20) | últimos eventos, para o snapshot inicial do SSE | 6 h após o fim |
| `matches:live` | ZSET (score = kickoff) | listagem de partidas ao vivo | — |
| `provider:{name}:tokens` | STRING | token bucket do rate limit | janela |

Nenhuma dessas chaves é fonte da verdade. Perder o Redis inteiro degrada latência e derruba conexões SSE; não perde dado. As chaves são reconstruíveis a partir do Postgres. Essa propriedade é o que torna o Redis operacionalmente simples: um cache que pode ser descartado exige muito menos cuidado do que um datastore que precisa ser protegido.

## 5. Caminho de leitura

### 5.1 Contratos

```
GET  /matches?status=live|scheduled|finished&competition=&date=
GET  /matches/{id}
GET  /matches/{id}/stream          # SSE
GET  /teams/{id}/matches?season=&cursor=
GET  /teams/{id}/stats?season=
GET  /players/{id}/stats?season=   # condicionado ao provider
GET  /healthz  /readyz  /metrics
```

O `web-service` **não consulta o Postgres para dado ao vivo**. `GET /matches/{id}` e o snapshot inicial do SSE saem integralmente do Redis. O Postgres entra no caminho de leitura em três situações, todas frias:

1. Histórico e estatísticas (FR-2) — dado imutável, cacheável na borda
2. Cache miss — reconstrói a chave a partir do log e reaquece o Redis
3. Gap-fill de reconexão SSE via `Last-Event-ID` — raro por conexão

### 5.2 SSE

```
event: snapshot
id: 42
data: {"match_id":"...","score":{"home":2,"away":1},"minute":67,...}

: heartbeat

event: goal
id: 43
data: {"type":"GOAL","seq":43,...}
```

Três detalhes sem os quais o SSE não funciona em condições reais:

- **Snapshot antes do stream, na mesma conexão.** Sem isso, o client abre o stream e encara uma tela vazia até o próximo evento — que pode não vir nos 40 minutos seguintes.
- **Heartbeat como comentário a cada 15 s.** Load balancers e proxies encerram conexões idle sem aviso. É a falha clássica de SSE em produção, e ela só aparece depois do deploy.
- **`Last-Event-ID` no reconnect.** O servidor consulta `match_events WHERE match_id = ? AND seq > ?` e preenche a lacuna antes de retomar o tempo real. Sem isso, a garantia de não perder dado terminaria no banco, não na tela — e o usuário perceberia a perda mesmo com o log íntegro.

Endpoints de histórico respondem com `ETag` e `Cache-Control`: partida encerrada é imutável, e é onde uma CDN faz o trabalho pesado sem custo de engenharia.

### 5.3 Escala do fanout

O gargalo do caminho de leitura **não** são as instâncias de aplicação. Uma conexão SSE ociosa em Go custa pouco mais que uma goroutine e um buffer, e 50 mil conexões por instância é uma meta realista.

O gargalo é o **fanout do pub/sub**. Um gol numa partida assistida por milhões vira um `PUBLISH` multiplicado pelo número de instâncias que assinam aquele canal. Com 200 instâncias, é uma escrita e duzentas entregas, por evento, concentradas nos mesmos instantes.

Mitigações, em ordem de custo crescente:

1. **Cache local in-process, TTL de ~1 s.** Cinquenta mil clientes na mesma instância pedindo o mesmo placar não precisam de cinquenta mil round-trips ao Redis. Reduz a pressão de leitura em ordens de grandeza pelo custo de um segundo de staleness — irrelevante para o produto.
2. **Assinatura seletiva.** Cada instância assina apenas os canais das partidas que seus clientes conectados estão assistindo, não todos.
3. **Redis Cluster com shard por `match_id`.** Isola a partida de audiência extrema num shard próprio, e o clássico problema de hot key deixa de afetar as demais.
4. **Fanout em dois níveis**, se as anteriores não bastarem — relays intermediários entre Redis e instâncias de borda.

As opções 1 e 2 entram na implementação. As 3 e 4 são documentadas com o gatilho de adoção e não implementadas, por não haver carga que as justifique.

## 6. Disponibilidade

Todos os serviços do caminho de leitura são stateless, atrás de load balancer, com graceful shutdown que drena conexões antes de encerrar. Kafka com RF 3 e `min.insync.replicas=2`. Consumidores em consumer group, com rebalanceamento automático.

O `ingestor` é a exceção e merece atenção: ele **precisa** ser ativo-único, porque duas réplicas polleando em paralelo duplicariam requisições e estourariam a cota do provider. Reconhecer esse ponto único de falha e resolvê-lo explicitamente é parte da entrega.

A solução é **leader election via advisory lock no Postgres**, com standby quente assumindo em menos de 30 segundos. Não em Redis: lock distribuído sobre Redis tem correção contestada e depende de premissas sobre relógio e pausas de GC que este projeto não pode garantir. O Postgres já está no caminho, é fortemente consistente e resolve o problema sem premissas adicionais. Detalhamento em [ADR-0006](../adr/0006-leader-election-em-postgres.md).

## 7. Durabilidade

A cadeia de garantias, ponta a ponta:

1. **Produtor** — `acks=all` com produtor idempotente. A escrita só é confirmada quando as réplicas mínimas a reconheceram.
2. **Consumidor** — commit manual do offset **depois** da persistência. Falha antes do commit resulta em reprocessamento, nunca em perda. Semântica at-least-once, assumida deliberadamente.
3. **Idempotência** — `UNIQUE (match_id, event_key)` torna o reprocessamento inócuo. O efeito observável é exactly-once, sem o custo de transações distribuídas.
4. **DLQ** — mensagem que falha repetidamente vai para tópico morto com o erro anexado, em vez de travar a partição indefinidamente.
5. **Fonte da verdade única** — Postgres. Redis é integralmente reconstruível.
6. **Última milha** — `Last-Event-ID` fecha a lacuna entre o banco e a tela.

At-least-once com idempotência, em vez de perseguir exactly-once no transporte, é uma escolha consciente: exactly-once real exigiria transações Kafka coordenadas com o banco, o que adiciona complexidade e modos de falha novos para obter uma garantia que uma constraint de unicidade já entrega neste domínio.

## 8. Observabilidade

Uma métrica define o produto:

> **end-to-end lag** — de `ingested_at` até a entrega no client

Instrumentada como trace OpenTelemetry através de `provider → ingestor → kafka → derivation → projector → redis publish → sse deliver`. Quando o lag sobe, o trace aponta **qual etapa** subiu. Essa é a diferença entre observabilidade e um painel bonito.

Métricas de apoio: consumer lag por group, orçamento de rate limit consumido, conexões SSE ativas por instância, profundidade da DLQ, taxa de 304 do provider.

O atraso entre o mundo real e o provider é invisível para o sistema e não entra na SLI. O intervalo de polling é exportado separadamente, como piso conhecido do atraso total.

## 9. Estratégia de teste

| Camada | Ferramenta | Alvo |
|---|---|---|
| Unidade | testes de tabela em Go | `internal/diff` — VAR, dado sujo, snapshot fora de ordem, placar retroativo, campos ausentes |
| Golden files | fixtures reais gravadas | sequência de snapshots do provider → sequência de eventos esperada |
| Integração | `testcontainers-go` | Kafka, Redis e Postgres reais, sem mock |
| Caos | script + compose | **prova do NFR-3** |
| Carga | k6 | teto de conexões SSE, latência de entrega |

O **teste de caos** é a peça central. Roteiro: simulador injeta eventos a taxa constante; no meio da execução, um consumidor é morto, depois um broker; ao final, o assert é que a contagem de eventos persistidos é exatamente igual à contagem injetada, sem duplicatas.

Isso converte "garanto zero perda de dados" de afirmação em evidência verificada por máquina, em CI. É o item de maior retorno de todo o plano de testes.

## 10. Slice AWS

O caminho de ingestão vai para serverless; o `web-service` fica em container. A assimetria é deliberada.

**A ingestão é serverless por natureza:** execução curta, sem estado, agendada, com carga muito irregular — rodada de campeonato versus terça-feira de madrugada. EventBridge Scheduler dispara uma Lambda em Go/arm64, que consulta o provider e publica no Kafka. Cabe folgadamente no free tier.

**SSE em Lambda é armadilha.** Function URLs suportam response streaming, então tecnicamente funciona. Mas o teto de 15 minutos por invocação força reconexão no meio do segundo tempo, e o modelo de cobrança é por duração — significa pagar por milhões de conexões **ociosas esperando um gol**. Uma conexão SSE é longa e barata em memória; Lambda cobra exatamente pelo recurso que ela consome, que é tempo. É o pior encaixe possível de modelo de custo. O `web-service` vai para Fargate atrás de ALB. Detalhamento em [ADR-0004](../adr/0004-sse-fora-do-lambda.md).

Duas práticas de IaC que custam zero e sinalizam maturidade: **remote state em S3 com lock em DynamoDB** — não state local commitado — e **GitHub Actions autenticando por OIDC**, sem access key de longa duração em secret.

MSK fica fora do slice por custo, com a decisão e o número registrados no README.

## 11. Estrutura do repositório

```
cmd/
  ingestor/  event-derivation/  score-projector/
  stats-projector/  web-service/  simulator/
internal/
  provider/     # interface Provider + adapters (footballdata, simulator)
  diff/         # função pura: (anterior, novo) -> []Event
  events/       # schema compartilhado
  kafka/  store/  cache/  sse/  config/  telemetry/
deploy/
  compose/      # stack local completa
  terraform/    # slice AWS
web/            # client mínimo
load/           # cenários k6
docs/
  adr/  specs/  guides/  runbook.md
```

O isolamento de `internal/diff`, sem qualquer dependência de I/O, é a decisão de organização mais consequente do projeto: torna a lógica mais difícil do sistema testável sem infraestrutura.

## 12. Fora de escopo, com justificativa

| Excluído | Motivo | Gatilho que reabriria |
|---|---|---|
| Kubernetes | Compose local e Fargate cobrem o caso. Seria tempo em YAML, não em arquitetura | Necessidade real de orquestração multi-serviço em escala |
| Kafka Streams / Flink | As projeções são `evento → upsert`. Consumidor simples resolve | Agregação com janela temporal ou join entre fluxos |
| Schema Registry | Um produtor, uma linguagem, um dono | Consumidor fora deste repositório |
| Event Sourcing formal | Há log de eventos e projeções, sem aggregates nem command handlers | Necessidade de reconstruir estado arbitrário no passado |
| Auth, contas, favoritos | Placar é dado público; não exercita nenhum NFR em jogo | Feature que dependa de identidade |
| Multi-região | Dobra complexidade e custo contra risco inexistente aqui | SLA real com requisito geográfico |
| Esports | Fase futura; `interface Provider` já é o ponto de extensão | Depois do MVP de futebol completo |

Saber onde a tecnologia **não** foi colocada é metade do valor deste documento.
