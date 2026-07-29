# Live Score

Sistema de acompanhamento de partidas de futebol em tempo real. Placar e eventos chegam à tela sem refresh, via Server-Sent Events, alimentados por um pipeline Kafka com garantias explícitas de durabilidade.

**Status:** em construção — Fase 0 de 7. Ver [roteiro](docs/guides/ROADMAP.md).

---

## Por que este projeto existe

É um projeto de estudo deliberado, com dois objetivos declarados: entregar um live score funcional, e servir de veículo para aprofundar Go, Kafka, Redis, Terraform e observabilidade a ponto de poder **defender cada decisão arquitetural**.

Consequência prática: todo o código executável é escrito manualmente. IA é usada como material de estudo, revisão e discussão — não como gerador de implementação. Um projeto de portfólio cujas decisões o autor não tomou não prova nada.

## O problema interessante

O produto é simples de descrever. A dificuldade está numa contradição aparente entre os requisitos:

- Escala de leitura para milhões de usuários simultâneos empurra o desenho para cache, réplicas e eventual consistency
- Durabilidade sem perda de dados empurra para escrita síncrona e fonte da verdade única

A contradição se dissolve ao perceber que as duas exigências não se aplicam à mesma coisa. **Ninguém precisa de consistência forte para *ver* um placar** — dois segundos de atraso são invisíveis. Precisa-se de consistência forte para **nunca perder um gol no log**.

| | Caminho de escrita | Caminho de leitura |
|---|---|---|
| Garantia | Durabilidade e ordem por partida | Eventualmente consistente |
| Meta | Zero eventos perdidos | p95 < 2 s até a tela |
| Escala com | Número de partidas no mundo | Audiência |
| Mecanismo | Kafka `acks=all`, idempotência, Postgres | Redis, SSE, cache local |

A assimetria que viabiliza isso é uma propriedade do domínio: **a escrita é limitada pela realidade do futebol, a leitura pela audiência.** Uma partida gera de 20 a 50 eventos em 90 minutos. O caminho de escrita, portanto, pode ser conservador e caro por evento — porque não cresce com usuários.

## Arquitetura

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
                                       │  SSE
                                       ▼
                                 client mínimo
```

## Três decisões que valem a leitura

**O provider entrega snapshot, não evento.** As APIs respondem *"a partida está 2×1 aos 67 minutos"*, nunca *"aconteceu um gol"*. Derivar eventos é responsabilidade do sistema, e esse diff mora num serviço separado consumindo o tópico bruto — porque é o trecho com maior probabilidade de bug, e preservar o dado bruto é o que permite corrigir o **passado** por replay. ([ADR-0002](docs/adr/0002-diff-engine-como-servico-separado.md))

**Partition key é `match_id`.** Kafka só garante ordem dentro da partição, e a única ordem que importa neste domínio é a de uma mesma partida — não há relação causal entre um gol no Maracanã e um cartão em Old Trafford. A chave entrega exatamente a garantia necessária, e nada além. ([ADR-0001](docs/adr/0001-particionamento-por-match-id.md))

**SSE não vai para Lambda.** Function URLs com streaming tornam isso tecnicamente possível, mas Lambda cobra por duração de execução, e uma conexão SSE é longa e quase inteiramente ociosa. O único recurso cobrado é exatamente o único recurso que ela consome em abundância. A ingestão, sim, é serverless por natureza — curta, sem estado, agendada. ([ADR-0004](docs/adr/0004-sse-fora-do-lambda.md))

## Stack

**Go** em todos os serviços · **Kafka** (KRaft) · **Redis** (cache, pub/sub, rate limiting) · **Postgres** (fonte da verdade) · **SSE** · **Terraform** · **OpenTelemetry**, Prometheus, Grafana · **k6** · `testcontainers-go`

## O que ficou de fora, e por quê

Metade do valor de um projeto está em onde a tecnologia **não** foi colocada.

| Excluído | Motivo | Gatilho que reabriria |
|---|---|---|
| Kubernetes | Compose e Fargate cobrem o caso. Seria tempo em YAML, não em arquitetura | Orquestração multi-serviço em escala real |
| Kafka Streams / Flink | As projeções são `evento → upsert` | Agregação com janela temporal ou join de fluxos |
| Schema Registry | Um produtor, uma linguagem, um repositório. O compilador já valida — de forma mais forte | Consumidor fora deste repositório |
| Event Sourcing formal | Há log e projeções, sem aggregates nem command handlers | Reconstruir estado arbitrário no passado |
| Auth e contas | Placar é dado público; não exercita nenhum NFR em jogo | Feature dependente de identidade |
| Multi-região | Dobra complexidade e custo contra risco inexistente | SLA com requisito geográfico |
| MSK | Acima de US$ 150/mês; fora do orçamento do slice | — |

## Documentação

| Documento | Conteúdo |
|---|---|
| [PRD](docs/PRD.md) | Requisitos, personas, critérios de aceite, riscos, métricas |
| [Design técnico](docs/specs/2026-07-28-live-score-design.md) | Arquitetura, modelo de dados, fanout, durabilidade, testes |
| [ADRs](docs/adr/) | Decisões com alternativas descartadas e consequências aceitas |
| [Roteiro](docs/guides/ROADMAP.md) | Plano de implementação fase a fase |

## Como rodar

A ser preenchido ao final da Fase 0.

## Provas planejadas

Os requisitos não funcionais serão verificados, não afirmados:

- **Teste de caos em CI** — sob injeção contínua de eventos, matar um consumidor e um broker resulta em contagem final exatamente igual à injetada, sem duplicatas. É o NFR de durabilidade verificado por máquina.
- **Relatório de carga** — teto medido de conexões SSE por instância, gargalo identificado (o fanout do pub/sub, não as instâncias de aplicação) e a extrapolação para 10M declarada explicitamente como extrapolação.

Nenhum número inventado. "Medi X, o gargalo é Y, aqui está a conta" é uma resposta mais forte que uma afirmação não verificável.

## Licença

MIT
