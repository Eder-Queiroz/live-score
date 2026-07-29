# ADR-0006 — Leader election do ingestor via advisory lock no Postgres

**Status:** aceito · **Data:** 2026-07-28

## Contexto

O `ingestor` não pode ser replicado de forma ativa-ativa. O rate limit do provider é da chave de API, não da instância: duas réplicas polleando em paralelo duplicariam as requisições, estourariam a cota e resultariam em HTTP 429 seguido de bloqueio — violando o NFR-5.

Isso faz do `ingestor` o único ponto único de falha do sistema. O NFR-2 exige que sua morte seja absorvida em menos de 30 segundos, o que demanda uma réplica standby e um mecanismo de eleição para garantir que apenas uma esteja ativa a qualquer momento.

O token bucket compartilhado no Redis limita o **consumo agregado** de cota, mas não resolve a eleição: duas réplicas ativas dividindo o mesmo orçamento reduziriam pela metade a frequência de polling de cada partida, degradando a latência do produto sem ganho algum.

## Decisão

Leader election por **advisory lock no Postgres** (`pg_try_advisory_lock`). A instância que obtém o lock é o líder e executa o loop de polling. As demais aguardam, tentando periodicamente. O lock é vinculado à sessão: se o processo morre ou a conexão cai, o Postgres o libera automaticamente.

## Alternativas consideradas

**Lock distribuído em Redis (`SET NX PX` ou Redlock).** O Redis já está no projeto, e é a solução mais divulgada. Rejeitada — ver análise abaixo.

**etcd ou Consul.** Ferramentas projetadas exatamente para isso, com consenso Raft e leases corretas. Rejeitada: adiciona um cluster de coordenação inteiro ao projeto para resolver uma eleição entre duas instâncias, quando um componente fortemente consistente já está no caminho.

**Kafka consumer group como mecanismo de eleição.** Um tópico de controle com uma partição elegeria naturalmente um único consumidor ativo. Criativo e sem dependência nova, mas rejeitado: usa o rebalanceamento do Kafka para uma finalidade que não é a sua, e o comportamento durante rebalance é mais difícil de raciocinar do que um lock explícito.

**Nenhuma eleição — instância única, aceitando o downtime.** Rejeitada: viola o NFR-2 e deixa o ponto único de falha sem tratamento, que é justamente o que se quer demonstrar como resolvido.

## Consequências

### Por que não Redis, mesmo estando à mão

A correção de lock distribuído sobre Redis é objeto de crítica técnica conhecida e não resolvida. O problema não é o Redis ser ruim, é a garantia ser incompatível com o que se pede dela:

- **Redis single-node com replicação assíncrona** pode perder o lock num failover. O primário confirma o lock, morre antes de replicar, e a réplica promovida não sabe que ele existe — duas instâncias se consideram líderes simultaneamente
- **Redlock** depende de premissas sobre sincronia de relógio e ausência de pausas longas de processo. Uma pausa de GC ou uma suspensão de VM pode fazer um líder acreditar que ainda detém um lock já expirado

Neste sistema, dois líderes simultâneos significam cota estourada e bloqueio pelo provider — exatamente a falha que a instância única existe para evitar. O modo de falha do mecanismo produziria o dano que ele deveria prevenir.

O Postgres, em contraste, é fortemente consistente, já está no caminho crítico da escrita, e advisory lock vinculado à sessão tem semântica simples de raciocinar: enquanto a conexão viver, o lock é seu; quando ela morrer, por qualquer motivo, ele é liberado. Não há premissa sobre relógio.

Nada disso significa que Redis seja escolha errada em geral. Significa que aqui o **custo do modo de falha** é alto o bastante para exigir a garantia mais forte, e ela está disponível de graça.

**Negativas aceitas**
- O `ingestor` passa a depender do Postgres para funcionar, mesmo não escrevendo nele. Aceitável: o Postgres é uma dependência dura do sistema como um todo
- Um lock de sessão exige conexão dedicada e viva; o líder precisa detectar perda de conexão e parar de pollear imediatamente, antes que o standby assuma
- Existe uma janela curta em que ambos podem se considerar líderes, entre a queda da conexão do antigo e sua detecção por ele mesmo. Mitigada pelo token bucket compartilhado, que limita o dano de cota nesse intervalo

O último ponto é a razão de manter os dois mecanismos: o lock previne o caso normal, o token bucket contém o caso de borda. Defesa em profundidade contra a única falha que causaria bloqueio pelo provider.

## Gatilho de revisão

Se o projeto adotar Kubernetes, `Lease` da coordination API passa a ser o mecanismo idiomático e substitui isso sem custo adicional de infraestrutura.
