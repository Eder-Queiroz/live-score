# ADR-0005 — Postgres como fonte da verdade, Redis como cache descartável

**Status:** aceito · **Data:** 2026-07-28

## Contexto

O PRD declara dois requisitos não funcionais em tensão: escala de leitura para 10 milhões de usuários simultâneos, e consistência forte sem perda de dados. O primeiro empurra o desenho para cache, réplicas e eventual consistency; o segundo, para fonte da verdade única e escrita síncrona durável.

É preciso decidir qual armazenamento detém a verdade e como o caminho de leitura é servido.

## Decisão

**Postgres é a única fonte da verdade.** `match_events` é append-only, com `UNIQUE (match_id, event_key)`.

**Redis é cache puro e integralmente descartável.** Guarda estado materializado da partida, cauda de eventos recentes, índice de partidas ao vivo e o token bucket do rate limit. Nenhuma dessas chaves é a verdade sobre nada.

O `score-projector` escreve nos dois: `INSERT` no Postgres para durabilidade, `HSET` no Redis para leitura, `PUBLISH` no Redis para fanout.

O `web-service` **não consulta o Postgres para dado ao vivo**.

## Alternativas consideradas

**Redis como fonte da verdade, com persistência AOF.** Leitura mais rápida, um armazenamento a menos. Rejeitada: AOF com `fsync` a cada escrita degrada drasticamente a performance, e com `fsync` por segundo há uma janela de perda. Um failover perderia gols permanentemente, violando o NFR-3 de forma direta. Além disso, transforma o Redis em componente que precisa ser protegido, o que é operacionalmente muito mais caro que um cache.

**Apenas Postgres, sem Redis.** Consistência trivial, um componente a menos. Rejeitada: cada uma das conexões SSE precisaria do estado atual da partida, e o pub/sub não existiria — seria preciso polling no banco ou `LISTEN/NOTIFY`, que não escala para o número de assinantes projetado.

**Cassandra ou DynamoDB para o log.** Escala de escrita superior. Rejeitada como sobre-engenharia: a escrita deste sistema é da ordem de dezenas por segundo, muito abaixo do ponto em que Postgres se torna limitante. Adotar um banco distribuído aqui custaria consistência forte e capacidade de query relacional em troca de escala que não será usada.

## Consequências

### A assimetria do domínio é o que torna isso possível

**A escrita é limitada pela realidade do futebol; a leitura, pela audiência.** Uma partida gera de 20 a 50 eventos em 90 minutos. Mesmo com centenas de partidas simultâneas no mundo, a escrita fica na casa de dezenas por segundo — volume que um Postgres modesto absorve sem esforço. Já a leitura são milhões de pessoas querendo o mesmo placar.

A consequência é que o Postgres está numa dimensão que **não cresce com o número de usuários**. Cresce com o número de partidas existentes no mundo, que é um número pequeno, conhecido e limitado. Por isso é possível ser conservador e caro por evento no caminho de escrita — escrita síncrona, durável, fortemente consistente — sem pagar preço de escala. Todo o custo de escala cai no caminho de leitura, que é exatamente onde eventual consistency é aceitável, porque dois segundos de atraso num placar são invisíveis ao usuário.

Cada requisito é atendido na camada onde é barato atendê-lo. É isso que resolve a contradição aparente dos NFRs.

### Cache descartável é uma propriedade operacional, não um detalhe

Como nenhuma informação existe **apenas** no Redis, perdê-lo por completo degrada latência e derruba conexões SSE, mas não perde dado. As chaves são reconstruíveis a partir do log. Isso significa que o Redis não precisa de backup, não precisa de política de retenção, não precisa de plano de recuperação, e um `FLUSHALL` acidental é um incidente de performance e não de integridade.

Um cache que pode ser jogado fora exige muito menos cuidado operacional que um datastore que precisa ser protegido. Essa diferença aparece em toda decisão subsequente de operação.

**Negativas aceitas**
- Escrita dupla no `score-projector`: sem transação distribuída, existe janela em que Postgres tem o evento e o Redis ainda não. Aceitável — a verdade está no Postgres, e o Redis converge no próximo evento ou na reconstrução por cache miss
- Dois armazenamentos para operar
- Uma reconstrução de cache é mais custosa que uma leitura normal, mas é rara e limitada a uma partida

## Gatilho de revisão

Se o volume de escrita crescer em ordens de grandeza — por exemplo, ingestão de dados posicionais de jogadores em alta frequência em vez de eventos discretos —, a premissa de que a escrita não escala com usuários deixa de valer, e o log precisaria migrar para armazenamento particionado.
