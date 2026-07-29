# Architecture Decision Records

Cada ADR registra uma decisão arquitetural com seu contexto, as alternativas descartadas e as consequências aceitas — incluindo as negativas.

O objetivo declarado deste diretório é ser defensável numa conversa técnica. Uma decisão sem alternativa considerada e sem consequência negativa listada não é uma decisão, é uma preferência.

| # | Decisão | Status |
|---|---|---|
| [0001](0001-particionamento-por-match-id.md) | Particionamento do Kafka por `match_id` | aceito |
| [0002](0002-diff-engine-como-servico-separado.md) | Diff engine como serviço separado, consumindo snapshots brutos | aceito |
| [0003](0003-sem-schema-registry.md) | Sem Schema Registry no MVP | aceito |
| [0004](0004-sse-fora-do-lambda.md) | SSE em container (Fargate), ingestão em Lambda | aceito |
| [0005](0005-postgres-fonte-da-verdade-redis-como-cache.md) | Postgres como fonte da verdade, Redis como cache descartável | aceito |
| [0006](0006-leader-election-em-postgres.md) | Leader election do ingestor via advisory lock no Postgres | aceito |

## Decisões previstas, ainda não tomadas

Serão escritas quando a fase correspondente for implementada, e não antes — um ADR escrito antes de o problema existir documenta especulação, não decisão.

| Tema | Fase |
|---|---|
| Biblioteca cliente de Kafka em Go (`franz-go` vs `segmentio/kafka-go` vs `confluent-kafka-go`) | 1 |
| Estratégia de migrations | 1 |
| SSE em vez de WebSocket ou long polling | 2 |
| Estratégia de retry e classificação de erro do provider | 3 |
| Formato de cursor na paginação de histórico | 4 |
| Mitigação escolhida para o gargalo de fanout, após medição | 5 |

## Convenção

Arquivos nomeados `NNNN-titulo-em-kebab-case.md`, numeração sequencial, nunca reaproveitada. Um ADR superado não é apagado nem editado: seu status muda para `superado por ADR-NNNN`. O histórico do raciocínio é o valor do registro — inclusive quando o raciocínio se mostrou errado.

Estrutura: **Contexto** (a força que exige a decisão) · **Decisão** · **Alternativas consideradas** (com o motivo da rejeição) · **Consequências** (positivas e negativas) · **Gatilho de revisão** (o que faria mudar de ideia).

O gatilho de revisão é o campo mais valioso e o mais frequentemente omitido. Ele é a diferença entre "escolhi X" e "escolhi X para estas condições, e sei reconhecer quando elas mudarem".
