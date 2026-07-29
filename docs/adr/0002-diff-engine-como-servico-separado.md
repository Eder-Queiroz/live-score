# ADR-0002 — Diff engine como serviço separado, consumindo snapshots brutos

**Status:** aceito · **Data:** 2026-07-28

## Contexto

As APIs de dados esportivos disponíveis no plano gratuito entregam **snapshot de estado**, não fluxo de eventos. A resposta é *"a partida está 2×1 aos 67 minutos, com estes cartões"*, e nunca *"acabou de acontecer um gol"*. Não há webhook em nenhum dos providers avaliados.

Portanto, derivar eventos discretos é responsabilidade do sistema: comparar o snapshot recém-obtido com o anterior e emitir `GOAL`, `CARD`, `SUBSTITUTION`, `SCORE_CORRECTED`. Resta decidir **onde** esse diff acontece.

## Decisão

O `ingestor` publica o snapshot bruto normalizado em `match.snapshots.v1`. Um serviço separado, `event-derivation`, consome esse tópico, executa o diff e publica os eventos derivados em `match.events.v1`.

A lógica de comparação vive em `internal/diff` como **função pura**, sem I/O: `(anterior, novo) → []Event`.

## Alternativas consideradas

**Diff dentro do `ingestor`, publicando apenas eventos.** Um serviço menos, um tópico menos, menor latência. Rejeitada pelo motivo desenvolvido abaixo: descarta o dado bruto.

**Diff no `score-projector`.** Reaproveita um serviço existente. Rejeitada: acopla derivação de eventos a persistência, e o `stats-projector` passaria a depender de eventos produzidos por outro consumidor, ou precisaria repetir a lógica.

**Persistir snapshots no Postgres e fazer o diff em batch.** Rejeitada: introduz latência incompatível com tempo real e cria uma tabela volumosa que nenhuma consulta de produto leria.

## Consequências

**Positivas**

A justificativa central é **capacidade de replay**. O diff é a parte do sistema com maior probabilidade de conter bug, porque é onde o mundo real é sujo: VAR anula gol, o provider corrige placar retroativamente, campos vêm ausentes ou trocados, um snapshot chega com estado impossível. Esses casos não são descobertos no desenvolvimento — são descobertos em produção, semanas depois, olhando uma timeline errada.

Com os snapshots brutos preservados no Kafka por 7 dias, corrigir é: ajustar `internal/diff`, resetar o offset do consumer group, reprocessar. O histórico é regenerado corretamente. Se o diff estivesse embutido no `ingestor`, o dado bruto nunca teria sido persistido e a correção só valeria para o futuro — o passado errado ficaria errado permanentemente, e não haveria como recuperá-lo, porque o provider não serve histórico de snapshots.

Ganhos secundários:
- `internal/diff` sendo função pura sem I/O, a parte mais complexa do sistema é testável sem Kafka, banco ou rede — testes de tabela cobrem VAR, dado sujo e ordem invertida em milissegundos
- Derivação e ingestão escalam de forma independente
- O `ingestor` fica reduzido a uma responsabilidade: buscar dado respeitando o rate limit

**Negativas**
- Um serviço e um tópico a mais para operar
- Um salto adicional de latência no caminho — na ordem de milissegundos, irrelevante contra um intervalo de polling de 15 segundos
- Armazenamento de snapshots brutos, mitigado por retenção de 7 dias

## Gatilho de revisão

Se algum provider passar a oferecer webhook de eventos discretos, o `event-derivation` deixa de ser necessário **para aquele provider** — o adapter passaria a publicar direto em `match.events.v1`. O tópico de snapshots continuaria existindo para providers que só entregam estado.
