# ADR-0004 — SSE em container (Fargate), ingestão em Lambda

**Status:** aceito · **Data:** 2026-07-28

## Contexto

Um dos objetivos de aprendizado do projeto é serverless com IaC. A tentação natural é colocar tudo em Lambda. É preciso decidir quais componentes vão para serverless e quais vão para container, e o critério dessa escolha é em si o aprendizado relevante.

Os dois componentes expostos são o `ingestor` (polling agendado) e o `web-service` (API HTTP mais conexões SSE de longa duração).

## Decisão

- **`ingestor` → Lambda (Go, arm64) disparada por EventBridge Scheduler**
- **`web-service` → ECS Fargate atrás de ALB**

## Alternativas consideradas

**Tudo em Lambda, com SSE via Function URL e response streaming.** Tecnicamente viável — Function URLs suportam streaming de resposta. Rejeitada pelas razões abaixo.

**Tudo em Fargate.** Uniforme e simples de operar. Rejeitada porque desperdiça o encaixe perfeito da ingestão em serverless, que é justamente o caso de uso canônico, e porque manter um container ligado 24 h para executar um poll a cada 15 segundos custa mais que Lambda.

**WebSocket via API Gateway.** API Gateway gerencia conexões WebSocket e resolveria o problema de conexão longa em serverless. Rejeitada: cobrança por minuto de conexão mais por mensagem, com o mesmo problema econômico descrito abaixo, além de exigir gerenciamento de connection IDs e fanout por API — substancialmente mais complexo que SSE para um fluxo que é unidirecional por natureza.

## Consequências

### Por que a ingestão encaixa em Lambda

O perfil é exatamente o que serverless resolve bem: execução curta, sem estado, agendada, com carga muito irregular. Um domingo de rodada tem dezenas de partidas ao vivo; uma terça-feira de madrugada tem zero. Pagar por container ligado nas duas situações é desperdício, e a Lambda cobra somente pelas invocações que ocorreram. Cabe folgadamente no free tier.

### Por que SSE em Lambda é armadilha

Dois problemas, e o segundo é decisivo.

**Teto de 15 minutos por invocação.** Uma conexão SSE numa partida dura os 90 minutos do jogo. O teto forçaria reconexão a cada 15 minutos — no meio do segundo tempo, para todos os clientes simultaneamente. O mecanismo de `Last-Event-ID` existe e absorveria isso sem perda de dado, mas seriam reconexões em massa sincronizadas, criando picos de carga artificiais e evitáveis.

**O modelo de cobrança é invertido em relação ao recurso consumido.** Lambda cobra por duração de execução. Uma conexão SSE é longa e quase inteiramente **ociosa** — passa a maior parte do tempo esperando um gol que acontece três vezes em 90 minutos. Em Go, essa conexão ociosa custa pouco mais que uma goroutine e um buffer, e uma instância sustenta dezenas de milhares delas concorrentemente. Em Lambda, cada conexão é uma invocação sendo cobrada pelo tempo inteiro em que espera sem fazer nada.

Ou seja: o único recurso que a Lambda cobra é exatamente o único recurso que uma conexão SSE consome em abundância. É o pior encaixe possível entre modelo de custo e perfil de carga. Na escala projetada pelo NFR-1, a diferença não é marginal — é ordens de grandeza.

Fargate cobra por vCPU e memória provisionados, que é o recurso que de fato limita o número de conexões concorrentes. O modelo de custo fica alinhado ao gargalo real.

**Negativas aceitas**
- Dois modelos de deploy no mesmo projeto, com dois conjuntos de recursos Terraform
- Fargate não é coberto pelo free tier; é a origem principal do custo mensal do slice, dimensionado e registrado no README

## Nota de aprendizado

Esta decisão é deliberadamente registrada como ADR porque o valor de portfólio está no raciocínio, não no resultado. Colocar tudo em Lambda demonstraria familiaridade com a ferramenta. Saber apontar onde ela é economicamente errada, e explicar por quê a partir do modelo de cobrança, demonstra julgamento — que é o sinal mais difícil de falsificar.

## Gatilho de revisão

Se a AWS remover o limite de duração de Function URLs com streaming **e** introduzir cobrança por conexão ociosa em vez de por duração de execução, o cálculo muda e a decisão deve ser reavaliada.
