# ADR-0001 — Particionamento do Kafka por `match_id`

**Status:** aceito · **Data:** 2026-07-28

## Contexto

Os eventos de partida trafegam por Kafka e são consumidos por múltiplos projectors. Kafka garante ordem apenas dentro de uma partição, nunca entre partições. É preciso decidir qual chave define a partição, e essa escolha determina quais garantias de ordem o sistema tem.

A ordem importa neste domínio: `GOAL` seguido de `SCORE_CORRECTED` significa gol anulado; a ordem invertida significa outra coisa completamente. Processar fora de ordem produz placar errado, e o PRD estabelece que placar errado é a falha mais grave possível do produto.

## Decisão

A partition key é `match_id`.

## Alternativas consideradas

**Chave aleatória ou round-robin.** Distribuição perfeitamente uniforme, paralelismo máximo. Rejeitada: destrói qualquer garantia de ordem, e nenhum consumidor conseguiria montar uma timeline confiável. Exigiria reordenação por `seq` no consumidor, com buffer e janela de espera — complexidade maior para resolver um problema que a chave certa elimina.

**`competition_id`.** Agruparia por campeonato. Rejeitada: garante mais ordem do que o necessário, ao custo de serializar partidas independentes do mesmo campeonato na mesma partição. Numa rodada com dez jogos simultâneos, todos disputariam um único consumidor.

**Partição única no tópico.** Ordem total trivial. Rejeitada: elimina paralelismo e o próprio motivo de usar Kafka.

## Consequências

**Positivas**
- Ordem total garantida dentro de cada partida, que é a única ordem semanticamente relevante
- Uma partida é sempre roteada para o mesmo consumidor, o que permite que `event-derivation` mantenha estado local com segurança — o snapshot anterior daquela partida
- Partidas distintas paralelizam livremente entre partições
- O consumidor não precisa de buffer de reordenação

**Negativas**
- Distribuição desigual: uma partida de audiência extrema gera partição mais quente
- O número de partições limita o paralelismo máximo e é difícil de aumentar depois sem quebrar a afinidade chave→partição

A partição quente é aceitável porque o volume de **escrita** por partida é minúsculo — 20 a 50 eventos em 90 minutos. Audiência gigantesca afeta o caminho de leitura, não o de escrita, e o caminho de leitura não passa por esta partição.

## Gatilho de revisão

Se uma única partida passar a gerar volume de escrita alto o suficiente para saturar um consumidor — por exemplo, com ingestão de dados posicionais de jogadores a cada segundo em vez de eventos discretos —, seria necessário sub-particionar por `(match_id, categoria_de_evento)`, aceitando perder ordem entre categorias.
