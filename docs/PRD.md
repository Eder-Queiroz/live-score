# PRD — Live Score

**Status:** aprovado · **Versão:** 1.0 · **Data:** 2026-07-28

---

## 1. Visão

Um sistema de acompanhamento de partidas de futebol em tempo real. O usuário abre a partida e vê placar, minuto e eventos (gols, cartões, substituições) se atualizarem na tela sem recarregar nada. Além do tempo real, consegue consultar o histórico e as estatísticas de partidas encerradas, times e jogadores.

O produto é simples de descrever de propósito. A dificuldade não está na feature — está em entregar essa feature com garantias fortes de durabilidade no caminho de escrita e com fanout para uma audiência muito grande no caminho de leitura, que são pressões que empurram o desenho em direções opostas.

## 2. Objetivos

Este projeto tem dois objetivos, e ambos são de primeira classe.

### 2.1 Objetivo de produto

Entregar um live score funcional, observável e testado, cujas garantias sejam verificadas por evidência e não por afirmação.

### 2.2 Objetivo de aprendizado

O projeto é um veículo de estudo deliberado das seguintes práticas:

| Tecnologia | O que precisa ser exercitado de verdade |
|---|---|
| **Go** | Concorrência, goroutines em conexões longas, testes de tabela, interfaces como ponto de extensão |
| **Kafka** | Particionamento e ordenação, consumer groups, offsets e commit manual, idempotência, DLQ, replay |
| **Redis** | Estruturas de dados, pub/sub, TTL, rate limiting distribuído, cache como recurso descartável |
| **Terraform** | Módulos, remote state, autenticação sem credencial de longa duração |
| **Serverless** | Onde encaixa (ingestão agendada) e, principalmente, onde **não** encaixa (conexões longas) |
| **Observabilidade** | Tracing distribuído com uma SLI que descreve o produto, não a máquina |

**Critério de sucesso do objetivo de aprendizado:** ser capaz de defender, numa entrevista técnica, cada decisão arquitetural registrada nos ADRs — incluindo as decisões de *não* adotar uma tecnologia.

Consequência prática: todo código executável do repositório é escrito manualmente pelo autor. IA é usada como material de estudo, revisão e discussão de arquitetura, nunca como gerador de implementação.

## 3. Não-objetivos

| Fora de escopo | Motivo |
|---|---|
| Contas de usuário, login, favoritos, notificações push | Placar é dado público. Auth não exercita nenhum dos NFRs em jogo |
| Apostas, odds, dados financeiros | Domínio regulado, desnecessário à tese técnica |
| App mobile nativo | O client existe apenas para provar o FR-1 visualmente |
| Cobertura multi-esporte no MVP | Esports entra depois, como prova de extensibilidade do `Provider` |
| Multi-região / disaster recovery geográfico | Dobra complexidade e custo contra um risco que o projeto não tem |
| Ser um produto comercial | Não há usuário real; os NFRs são metas de engenharia, não SLA contratual |

## 4. Personas

**Torcedor de segunda tela.** Está vendo o jogo, ou não conseguiu ver, e quer o placar imediato. Tolerância a atraso: baixa — alguns segundos. Tolerância a erro: **zero**. Um placar errado destrói a confiança no produto de forma que um placar atrasado não destrói. Isso ordena as prioridades: correto primeiro, rápido em segundo.

**Torcedor analítico.** Quer o histórico do time, o retrospecto, a estatística do jogador. Tolerância a atraso: alta. Consulta dado imutável, e portanto agressivamente cacheável.

**Avaliador técnico (recrutador ou entrevistador).** Stakeholder real e declarado deste projeto. Precisa entender a arquitetura em poucos minutos, encontrar as decisões documentadas e ver evidência de que as garantias afirmadas foram verificadas. É esta persona que justifica o peso dado a ADRs, ao relatório de carga e ao teste de caos.

## 5. Requisitos funcionais

### FR-1 — Placar ao vivo com atualização automática

O usuário visualiza o estado corrente de uma partida e recebe atualizações sem qualquer ação sua.

**Critérios de aceite**
- `GET /matches/{id}/stream` mantém conexão SSE aberta e entrega eventos conforme ocorrem
- O primeiro frame da conexão é um `snapshot` do estado atual, na mesma conexão — a tela nunca começa vazia
- O client reflete a mudança sem recarregar a página
- A conexão sobrevive a períodos sem eventos (heartbeat)
- Uma partida sem eventos por 20 minutos mantém a conexão viva

### FR-2 — Histórico e estatísticas

O usuário consulta dados de partidas encerradas, de times e de jogadores.

**Critérios de aceite**
- `GET /matches/{id}` devolve o estado final e a timeline completa de uma partida encerrada
- `GET /teams/{id}/matches` devolve o histórico paginado por cursor
- `GET /teams/{id}/stats` devolve agregados da temporada
- `GET /players/{id}/stats` devolve agregados por jogador **— condicionado à disponibilidade no plano gratuito do provider (ver Risco R2)**
- Respostas de dado imutável trazem `ETag` e `Cache-Control`

### FR-3 — Listagem de partidas

**Critérios de aceite**
- `GET /matches` filtra por `status` (`live`, `scheduled`, `finished`), `competition` e `date`
- A listagem de partidas ao vivo é servida de cache e não consulta o banco

### FR-4 — Timeline minuto a minuto

**Critérios de aceite**
- A timeline é ordenada de forma estável e determinística por `seq`
- Contém no mínimo: gol, cartão, substituição, mudança de período, correção de placar
- Uma correção de placar (VAR) aparece como evento próprio, sem apagar o gol original

### FR-5 — Reconexão sem lacuna

**Critérios de aceite**
- O client que reconecta enviando `Last-Event-ID` recebe todos os eventos com `seq` maior que o informado, antes de voltar ao tempo real
- Nenhum evento é entregue em duplicidade ao client após a reconexão
- Derrubar a rede do client por 2 minutos durante uma partida ativa resulta em timeline final idêntica à de um client que ficou conectado

## 6. Requisitos não funcionais

Os NFRs originais foram declarados como "10 milhões de usuários simultâneos", "alta disponibilidade" e "consistência forte, sem perda de dados". A tensão entre o primeiro e o terceiro é real e é resolvida separando os planos do sistema — decisão detalhada no design técnico e no ADR-0005.

### NFR-1 — Escala de leitura

**Meta declarada:** 10 milhões de conexões simultâneas em dia de jogo grande.

**Como é tratado:** este número não será provado num projeto de portfólio, e afirmar o contrário seria desonesto. A entrega é composta de três partes:

1. **Medição real** do teto de conexões SSE por instância, na máquina de desenvolvimento, com k6
2. **A conta explícita** de quantas instâncias o número medido implica para 10M
3. **Identificação do gargalo real**, que não são as instâncias de aplicação e sim o fanout do pub/sub, com a mitigação documentada

**Critério de aceite:** relatório em `docs/load-test.md` com número medido, metodologia, gargalo identificado e extrapolação declarada como extrapolação.

**Piso aceitável:** 10.000 conexões SSE simultâneas por instância. Alvo: 50.000.

### NFR-2 — Disponibilidade

**Meta:** o caminho de leitura não tem ponto único de falha; o caminho de escrita tolera perda de qualquer instância sem perder evento.

**Critérios de aceite**
- Todos os serviços do caminho de leitura são stateless e escalam horizontalmente
- Reinício em rolling de qualquer serviço não derruba requisição em andamento (graceful shutdown com drain de conexões)
- Kafka com replication factor 3 e `min.insync.replicas=2`
- O ingestor, que é necessariamente ativo-único, elege líder e tem standby quente; a morte do líder é absorvida em menos de 30 segundos

### NFR-3 — Durabilidade e consistência

**Meta:** nenhum evento de partida é perdido, em nenhuma hipótese de falha coberta.

**Critérios de aceite**
- Produtor Kafka com `acks=all` e idempotência habilitada
- Offset comitado somente após persistência bem-sucedida
- Constraint de unicidade que torna reprocessamento inócuo
- Postgres é a única fonte da verdade; Redis é reconstruível integralmente a partir dele
- **Teste de caos automatizado em CI**: sob injeção contínua de eventos, matar um consumidor e um broker resulta em contagem final de eventos persistidos exatamente igual à contagem injetada, sem duplicatas

O último critério é o mais importante do documento. Ele converte o NFR-3 de afirmação em evidência reproduzível.

### NFR-4 — Latência

**SLI:** `ingested_at → sse_delivered_at` — o tempo dentro da fronteira do sistema.

| Percentil | Meta |
|---|---|
| p50 | < 500 ms |
| p95 | < 2 s |
| p99 | < 5 s |

O atraso do provider até a realidade **não é medível** por este sistema e portanto não entra na SLI. O intervalo de polling é registrado à parte como piso conhecido de atraso total.

### NFR-5 — Respeito ao rate limit do provider

**Critério de aceite:** zero respostas HTTP 429 em 24 horas de operação contínua, com múltiplas réplicas do ingestor ativas.

## 7. Restrições externas

| Provider | Limite | Papel |
|---|---|---|
| football-data.org | 10 req/min (free) | Provider primário de futebol |
| thesportsdb.com | 30 req/min (free) | Alternativa avaliada; fallback |
| pandascore.co | 1000 req/h | Esports, fase futura |

Três restrições decorrem disso e moldam o desenho:

1. **Não há webhook.** A ingestão é polling. Não é escolha de arquitetura, é imposição do provider.
2. **A API entrega snapshot de estado, não fluxo de eventos.** Derivar os eventos é responsabilidade do sistema.
3. **O volume real é baixo** — dezenas de eventos por minuto no mundo inteiro. Insuficiente para exercitar o pipeline. Daí a existência do simulador como segunda implementação da mesma interface.

## 8. Métricas de sucesso do projeto

| # | Métrica | Meta |
|---|---|---|
| 1 | FR-1 demonstrável com dado real de partida real | Sim |
| 2 | Teste de caos provando NFR-3, rodando em CI | Verde |
| 3 | Conexões SSE simultâneas medidas | ≥ 10.000 por instância |
| 4 | 429 do provider em 24h de operação | 0 |
| 5 | ADRs escritos, cada um com contexto, decisão, alternativas e consequências | ≥ 6 |
| 6 | Custo mensal da infraestrutura AWS provisionada | < US$ 5 |
| 7 | Um leitor técnico entende a arquitetura pelo README | < 5 min |

## 9. Riscos

| ID | Risco | Impacto | Mitigação |
|---|---|---|---|
| R1 | Provider muda API, restringe o free tier ou sai do ar | Alto | Toda integração atrás de `interface Provider`; simulador garante que a demo nunca depende de terceiro |
| R2 | Estatística de jogador indisponível no plano gratuito | Médio | FR-2 declara esse item como condicional; time e competição são o núcleo garantido |
| R3 | Escopo cresce e o projeto não termina | Alto | Faseamento cortável pelo fim: as fases 0–2 já constituem entrega defensável |
| R4 | Custo AWS inesperado | Médio | Slice mínimo, budget alarm, `terraform destroy` documentado no runbook |
| R5 | Curva de aprendizado de Go/Kafka atrasa o cronograma | Médio | Aceito e desejado — é o objetivo declarado do projeto. Prazo é flexível; profundidade não |
| R6 | Sobre-engenharia por entusiasmo com as ferramentas | Médio | Lista de exclusões explícitas na seção 3 e no README, cada uma com o gatilho que reabriria a decisão |

## 10. Escopo por fase

| Fase | Entrega | Requisitos atendidos |
|---|---|---|
| 0 | Fundação: repo, compose, CI | — |
| 1 | Caminho de escrita ponta a ponta com simulador | Base de FR-4, NFR-3 |
| 2 | Caminho de leitura: API, SSE, client | **FR-1**, FR-3, FR-5 |
| 3 | Provider real, rate limit, leader election | FR-1 com dado real, NFR-5, NFR-2 |
| 4 | Estatísticas e histórico | **FR-2**, FR-4 |
| 5 | Observabilidade, teste de caos, load test | **NFR-1, NFR-3, NFR-4** |
| 6 | Slice AWS com Terraform | Objetivo de aprendizado |
| 7 | Documentação e apresentação | Métrica 7 |

## 11. Referências

- Design técnico: [`docs/specs/2026-07-28-live-score-design.md`](specs/2026-07-28-live-score-design.md)
- Decisões arquiteturais: [`docs/adr/`](adr/)
- Roteiro de estudo e implementação: [`docs/guides/ROADMAP.md`](guides/ROADMAP.md)
