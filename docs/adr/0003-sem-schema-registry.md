# ADR-0003 — Sem Schema Registry no MVP

**Status:** aceito · **Data:** 2026-07-28

## Contexto

Os eventos trafegam por Kafka entre serviços distintos. Schema Registry com Avro ou Protobuf é a prática de mercado dominante para governar a evolução desses contratos, e o projeto tem entre seus objetivos o contato com práticas de mercado.

## Decisão

Não adotar Schema Registry no MVP. O contrato dos eventos é definido por structs Go em `internal/events`, serializado como JSON, e versionado no nome do tópico (`match.events.v1`).

## Alternativas consideradas

**Confluent Schema Registry com Avro.** Prática dominante, validação de compatibilidade automática, payload binário compacto. Rejeitada para o MVP pelas razões abaixo.

**Protobuf sem registry.** Contrato explícito em `.proto`, geração de código, payload compacto — sem o container extra. Alternativa mais próxima de ser adotada. Rejeitada por ora: adiciona um passo de codegen ao build para resolver um problema de interoperabilidade entre linguagens que não existe hoje.

## Consequências

Schema Registry resolve um problema específico e bem definido: **evolução de contrato entre times, repositórios e linguagens que não se coordenam**. O produtor de um time altera o schema, o consumidor de outro time quebra em produção, e ninguém percebeu no code review porque os dois vivem em repositórios diferentes. O registry intercepta isso validando compatibilidade no momento da publicação.

Nenhuma dessas condições existe aqui. Há um produtor, uma linguagem, um repositório, um dono. O compilador de Go já é o mecanismo de validação: alterar uma struct em `internal/events` de forma incompatível quebra o build de todos os consumidores, no mesmo commit, antes do merge. Isso é uma verificação **mais forte** que a do registry — acontece em tempo de compilação, não em tempo de execução.

Adotar registry neste contexto significaria um container a mais no compose, um serviço a mais para manter disponível, um passo a mais no pipeline de build, e uma dependência a mais no caminho de escrita — em troca de zero garantia nova.

**Negativas aceitas**
- JSON é mais verboso que Avro. Irrelevante no volume deste sistema — dezenas de eventos por segundo
- Não há validação de compatibilidade automatizada; a disciplina recai sobre o code review
- Um consumidor externo escrito em outra linguagem precisaria reimplementar o schema a partir da documentação

## Gatilho de revisão

Qualquer uma das condições abaixo reabre a decisão, e a resposta passa a ser sim:

1. Um consumidor fora deste repositório, ou em outra linguagem
2. Necessidade de garantir compatibilidade retroativa sem controlar todos os consumidores
3. Volume em que o overhead de serialização JSON se torne mensurável no perfil de CPU

Enquanto nenhuma delas ocorrer, adicionar registry seria complexidade sem contrapartida. A capacidade de identificar isso é o que esta decisão registra.
