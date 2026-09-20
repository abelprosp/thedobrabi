# Análise técnica do TheDobra

Data: 19/09/2026. Escopo: repositório local em `/Users/arturabel/Documents/bi`.

## Estado das correções (19/09/2026)

Os achados P0/P1 e a maioria dos P2 críticos abaixo foram corrigidos no código. Aplicar as migrations `019_share_expiry.sql` e `020_cdc_lease.sql` antes de implantar. O que ainda fica como evolução (não bloqueador de segurança imediata): reescrita SAML completa com criptografia real, exportação assíncrona paginada, refatoração de componentes grandes do frontend, OpenAPI completo, e testes E2E multi-tenant no browser.

| ID | Status |
| --- | --- |
| 01 Convites | Corrigido — prova de identidade + MFA/sessão; tokens só após autenticação |
| 02 SAML | Mitigado — ACS desabilitado (501); parser recusa asserções sem crypto |
| 03 Markdown XSS | Corrigido — escape HTML antes da formatação |
| 04 Flows tenant | Corrigido — org/ws no SQL e handlers |
| 05 RLS uniforme | Corrigido — preview/export/joins; falha se regras não carregam |
| 06 Viewer write | Corrigido — matriz requireAnalyst/requireAdmin |
| 07 Share público | Corrigido — query pelo widget_id; expiry/revogação |
| 08 Replace atômico | Corrigido — staging + swap |
| 09 CDC | Corrigido — lease + cursor composto (limits documentados) |
| 10 Flow false success | Corrigido |
| 11 Entitlements | Corrigido |
| 12 Stripe webhook | Corrigido — transação idempotente |
| 13 Compose bind | Corrigido — 127.0.0.1 |
| 14 Reset sessões | Corrigido — revoga refresh tokens |
| 15 Testes ingest | Corrigido — cliente HTTP injetável |
| 16 Gateway | Corrigido |
| 17 Cache | Corrigido — role/RLS/versão na chave |
| 18 Truncamento | Parcial — headers X-Export-Truncated |

---

O projeto tem uma base funcional ampla, mas a prioridade deve ser fechar falhas de autenticação, autorização e integridade dos dados antes de ampliar o uso em produção. Há problemas concretos que a compilação e os testes atuais não detectam.

Esta revisão inventariou o repositório inteiro e aprofundou os caminhos de autenticação, permissões, consultas, compartilhamento, ingestão, flows, agendamento, cobrança, frontend e implantação. Não equivale a testar cada tela, integração externa ou combinação de configuração. O relatório original não alterava código; as correções posteriores estão no estado acima.

## Escopo e validação

O inventário encontrou 348 arquivos versionados, incluindo 123 arquivos Go, 74 TSX, 21 TS, 2 Python e 19 SQL. A arquitetura combina API Go, Next.js, PostgreSQL, ClickHouse, Redis, MinIO, Redpanda, gateway e worker Python.

| Verificação | Resultado |
| --- | --- |
| TypeScript do frontend, sem emitir arquivos | Passou |
| Compilação de produção Next.js em cópia temporária | Passou; Next.js instalado 15.5.25 |
| `go test ./...` da API | 11 pacotes passaram; ingestão apresentou 15 testes com falha; 21 pacotes sem testes |
| `go test ./...` do gateway | Compilou; nenhum teste existente |
| `go test ./...` do gerador de dados | Compilou; nenhum teste existente |
| Reprodução local do parser SAML original | Aceitou uma asserção sintética sem assinatura criptográfica válida |
| Reprodução local do formatador Markdown original | Preservou HTML com atributo de evento |
| Reprodução local de duas funções do gateway original | Confirmou driver PostgreSQL incorreto e duplicação de `LIMIT` |

As reproduções usaram arquivos temporários e dados sintéticos. Não houve exploração de contas, acesso ao ambiente de produção ou alteração de bancos. O worker foi revisado por leitura, sem validação integrada com Redis. Não foi realizada auditoria visual no navegador, teste de carga ou varredura de vulnerabilidades de dependências.

## Aspectos que vale preservar

- O monólito modular é uma organização razoável para o estágio atual. Os problemas encontrados podem ser corrigidos sem uma migração ampla para microsserviços.
- O caminho principal de consultas usa modelo semântico e valida identificadores, em vez de executar diretamente o SQL produzido pela IA.
- A API revalida a associação do usuário à organização/workspace em cada requisição autenticada.
- Há criptografia de configurações de conectores, hash de tokens, MFA, expiração de embeds, limites de consulta e timeout.
- O cliente HTTP de conectores já bloqueia destinos internos e resolve o endereço antes da conexão. Essa proteção deve ser preservada ao corrigir os testes.
- O agendador usa `FOR UPDATE SKIP LOCKED`, e a implantação contém readiness/liveness, limites de recursos e execução sem root.
- O frontend tem componentes comuns, estados de carregamento/erro, tema e cuidados de responsividade. Sua compilação está saudável.

## Achados prioritários

P0 significa correção imediata antes de exposição a usuários não confiáveis; P1 significa alto impacto em segurança, dados ou operação; P2 significa melhoria importante de robustez e manutenção. Os impactos de implantação dependem de essas configurações estarem efetivamente em uso.

### 01 — P0 — Convites podem autenticar usuários já existentes

**Evidência:** [retorno do link ao administrador](/Users/arturabel/Documents/bi/services/api/internal/apihttp/collab.go:163), [aceitação de convite](/Users/arturabel/Documents/bi/services/api/internal/authn/extra.go:184), [resolução da organização do usuário](/Users/arturabel/Documents/bi/services/api/internal/authn/service.go:226).

O administrador que cria um convite recebe `invite_url`. Ao aceitar esse link, se o e-mail já pertence a um usuário, o serviço não verifica sua senha, sessão existente ou MFA. Em seguida, emite tokens para esse usuário. A resolução usa `uuid.Nil` como workspace e pode selecionar sua organização mais antiga, não a organização do convite.

**Impacto:** um administrador de uma organização pode obter uma sessão de uma conta de outra organização ao convidar seu e-mail. O risco decorre da combinação desses caminhos, confirmada por leitura; não foi reproduzido contra um banco.

**Melhoria:** para contas existentes, exigir autenticação da própria pessoa, incluindo MFA, antes de aceitar o convite. O token deve autorizar somente a associação à organização convidante, nunca substituir autenticação. Fazer consumo do convite e associação em transação; resolver explicitamente a organização de destino. Testar administrador de A convidando usuário de B.

### 02 — P0 — SAML aceita assinatura apenas pela presença de texto

**Evidência:** [parser SAML](/Users/arturabel/Documents/bi/services/api/internal/sso/saml.go:52), [endpoint ACS](/Users/arturabel/Documents/bi/services/api/internal/apihttp/enterprise.go:81), [vinculação por e-mail](/Users/arturabel/Documents/bi/services/api/internal/authn/service.go:294).

O parser procura a palavra `Signature` no XML, mas não verifica assinatura com certificado confiável. O ACS não carrega nem valida a conexão SAML da organização indicada na URL. Não há validação de emissor, destinatário, validade temporal, correlação da solicitação ou repetição da resposta nesse caminho. Depois disso, a identidade pode ser vinculada a um usuário existente pelo e-mail informado.

**Validação:** a função original aceitou uma resposta sintética com elemento `Signature` vazio e retornou a identidade escolhida no teste.

**Melhoria:** desabilitar o ACS até haver validação completa por implementação SAML apropriada. Vincular provedor, certificado, organização e identidade; impedir associação automática a contas existentes com informação não verificada. Cobrir assinatura inválida, resposta expirada, emissor de outra organização e reutilização de resposta.

### 03 — P0 — Widgets Markdown permitem execução de HTML não confiável

**Evidência:** [renderização HTML](/Users/arturabel/Documents/bi/apps/web/src/components/WidgetView.tsx:300), [formatador](/Users/arturabel/Documents/bi/apps/web/src/components/WidgetView.tsx:976), [tokens no navegador](/Users/arturabel/Documents/bi/apps/web/src/lib/api.ts:23).

`renderMarkdown` faz substituições de texto e mantém HTML recebido. O resultado entra em `dangerouslySetInnerHTML`. O conteúdo é persistido no layout do dashboard e pode ser aberto por outros usuários, inclusive em links compartilhados. Os tokens de acesso e renovação estão no `localStorage`.

**Impacto:** XSS persistente, com possibilidade de executar ações na sessão de quem abre o dashboard e acessar tokens disponíveis ao JavaScript. A preservação de atributo de evento foi reproduzida localmente; não foi executado conteúdo malicioso em uma sessão real.

**Melhoria:** usar renderização Markdown com HTML bruto desabilitado ou sanitização estrita. Adicionar testes de conteúdo hostil, política de conteúdo na camada web e uma estratégia de sessão que reduza exposição de tokens ao JavaScript. Corrigir a renderização é indispensável mesmo se os tokens forem movidos para cookies.

### 04 — P1 — Etapas e execuções de flows não validam o tenant

**Evidência:** [handlers de etapas](/Users/arturabel/Documents/bi/services/api/internal/apihttp/arch_mvp.go:148), [handlers de execuções](/Users/arturabel/Documents/bi/services/api/internal/apihttp/arch_mvp.go:244), [persistência](/Users/arturabel/Documents/bi/services/api/internal/flow/store.go:239).

Listar/criar etapas utiliza apenas `flow_id`; atualizar/excluir utiliza apenas `step_id`. Os handlers não verificam a organização/workspace do flow, e o SQL não faz essa verificação. O mesmo ocorre na leitura de execuções e logs. O middleware autentica a pessoa, mas não estabelece que ela possui acesso ao objeto solicitado.

**Impacto:** um usuário autenticado que obtenha um identificador pode ler ou alterar objetos de outro cliente. UUID não substitui autorização.

**Melhoria:** fazer os métodos receberem organização, workspace e identificador do pai; validar a relação etapa → flow → tenant no próprio SQL. Aplicar o mesmo padrão a execuções/logs. Testar operações de A contra todos os objetos de B.

### 05 — P1 — As regras de acesso por linha não são aplicadas de forma uniforme

**Evidência:** [leitura e preview](/Users/arturabel/Documents/bi/services/api/internal/queryeng/engine.go:298), [exportação](/Users/arturabel/Documents/bi/services/api/internal/apihttp/export.go:17), [erro de RLS ignorado](/Users/arturabel/Documents/bi/services/api/internal/queryeng/engine.go:521), [joins](/Users/arturabel/Documents/bi/services/api/internal/queryeng/engine.go:535).

`ReadRows` e `Preview` filtram organização, mas não recebem usuário/papel nem aplicam RLS. A exportação e a rota de linhas estão disponíveis a usuários autenticados e usam esse caminho. Nas consultas semânticas, falha ao carregar RLS é ignorada. As regras são carregadas para o dataset principal, sem aplicação equivalente aos datasets dos joins.

**Impacto:** uma pessoa restrita a determinadas linhas pode conseguir acessá-las por outro endpoint; uma falha de leitura das permissões pode produzir consulta menos restrita.

**Melhoria:** centralizar autorização de leitura, aplicar RLS em preview, exportação, joins, IA e flows, e interromper a consulta quando não for possível avaliar as regras. Distinguir explicitamente operações internas autorizadas de consultas de usuário.

### 06 — P1 — Papel viewer permite operações de escrita e publicação

**Evidência:** [salvar dashboard](/Users/arturabel/Documents/bi/services/api/internal/apihttp/handlers.go:582), [excluir flow](/Users/arturabel/Documents/bi/services/api/internal/apihttp/arch_mvp.go:132), [gerar compartilhamento público](/Users/arturabel/Documents/bi/services/api/internal/apihttp/collab.go:221), [portal de cobrança](/Users/arturabel/Documents/bi/services/api/internal/apihttp/enterprise.go:177).

Esses handlers descartam o papel retornado por `principal` e não aplicam `requireAdmin`/`requireAnalyst`. O roteador exige autenticação, sem restrição adicional por papel nessas rotas.

**Melhoria:** definir uma matriz de permissões por ação e aplicá-la no backend. Viewer deve ter acesso de leitura; criação/edição, publicação externa, execução de pipelines e cobrança precisam de permissões explícitas. Testar a matriz nos handlers reais, não somente nas funções auxiliares.

### 07 — P1 — Link público libera consultas além do widget publicado

**Evidência:** [consulta pública](/Users/arturabel/Documents/bi/services/api/internal/apihttp/collab.go:274), [consulta de embed](/Users/arturabel/Documents/bi/services/api/internal/apihttp/embed.go:216), [joins automáticos](/Users/arturabel/Documents/bi/services/api/internal/queryeng/engine.go:95).

O backend verifica se o dataset está no layout, mas aceita medidas, dimensões e filtros enviados pelo visitante. A consulta usa usuário nulo, que pula RLS. Além disso, relacionamentos podem acrescentar joins automaticamente depois da validação dos joins explícitos.

**Impacto:** publicar um total filtrado pode dar acesso a consultas mais amplas sobre a fonte. A segurança atual está no nível do dataset, não no conteúdo que a pessoa decidiu publicar.

**Melhoria:** receber o identificador do widget e construir a consulta a partir de uma configuração autorizada no servidor. Permitir somente filtros explicitamente publicados, aplicar restrições fixas e validar todos os datasets após resolver relacionamentos. Acrescentar gestão de expiração/revogação também aos links de compartilhamento comuns.

### 08 — P1 — Substituição de dados apaga a versão anterior antes de validar a nova

**Evidência:** [replace de dataset](/Users/arturabel/Documents/bi/services/api/internal/ingest/update.go:246), [atualização por conector](/Users/arturabel/Documents/bi/services/api/internal/ingest/engine.go:451), [inserção em lotes](/Users/arturabel/Documents/bi/services/api/internal/ingest/engine.go:227).

O fluxo executa `TRUNCATE` e depois insere os dados em lotes. Se ocorrer falha ou timeout após a limpeza, o conjunto anterior já foi perdido e o novo pode ficar parcial. Alguns erros de atualização dos metadados também são ignorados.

**Melhoria:** carregar em uma tabela temporária/versionada, validar contagem, esquema e qualidade, e só então trocar a versão ativa de forma atômica. Manter a versão anterior recuperável e impedir duas substituições concorrentes do mesmo dataset.

### 09 — P1 — CDC pode duplicar e perder registros

**Evidência:** [loop por instância](/Users/arturabel/Documents/bi/services/api/internal/apihttp/server.go:93), [seleção sem exclusão mútua](/Users/arturabel/Documents/bi/services/api/internal/cdc/engine.go:119), [inserção e checkpoint](/Users/arturabel/Documents/bi/services/api/internal/cdc/engine.go:201), [cursor incremental](/Users/arturabel/Documents/bi/services/api/internal/ingest/engine.go:512), [três réplicas de API](/Users/arturabel/Documents/bi/infra/kubernetes/api.yaml:11).

Todas as réplicas consultam os mesmos checkpoints e podem inserir o mesmo lote. A escrita no ClickHouse e o avanço do cursor não têm deduplicação idempotente. O cursor usa somente `coluna > último_valor`: quando um lote termina no meio de vários registros com o mesmo timestamp, o próximo lote pula os restantes. Alterações de registros existentes são acrescentadas, e exclusões não são propagadas nesse caminho.

**Melhoria:** reivindicar cada checkpoint com lease/lock, usar cursor composto `(updated_at, chave_primária)` e projetar reprocessamento idempotente. Definir semântica de atualização/exclusão e recuperação depois de falha entre inserção e checkpoint. Testar duas réplicas, timestamps repetidos e reinício durante um lote.

### 10 — P1 — Flows podem informar sucesso sem produzir o resultado

**Evidência:** [materialização e conclusão](/Users/arturabel/Documents/bi/services/api/internal/flow/engine.go:143).

Falha na criação do dataset de saída é registrada no log, mas a execução ainda é marcada como `completed` e retorna sem erro. A validação de linhas registra a quantidade de problemas e continua sem uma política explícita de reprovação.

**Melhoria:** propagar erro de materialização, marcar a execução como falha e atualizar o agendamento correspondente. Definir quando a validação bloqueia publicação, apenas avisa ou envia linhas inválidas para quarentena. O status mostrado na interface deve representar o resultado persistido.

### 11 — P1 — Limites comerciais e catálogo de planos estão inconsistentes

**Evidência:** [condição de limite](/Users/arturabel/Documents/bi/services/api/internal/entitlements/entitlements.go:241), [função identidade](/Users/arturabel/Documents/bi/services/api/internal/entitlements/entitlements.go:290), [criação de dashboard](/Users/arturabel/Documents/bi/services/api/internal/apihttp/handlers.go:539), [catálogo de cobrança](/Users/arturabel/Documents/bi/services/api/internal/billing/stripe.go:33).

`lim.Dashboards < mar0(lim.Dashboards)` compara o valor com ele próprio e é sempre falso. A criação manual de dashboard também não chama essa verificação. A cobrança divulga Starter/Growth/Business/Enterprise com limites diferentes do catálogo efetivo Essencial/Pro/Completo. Por exemplo, os limites apresentados de usuários e créditos de IA não correspondem aos limites aplicados após normalização.

**Melhoria:** usar um catálogo único para preço, identificador Stripe, limites e UI; chamar as verificações em todos os caminhos de criação. Reservar/consumir cotas de forma atômica e medir custo de IA por operação, incluindo geração de visual, medida e análise, não apenas quantidade de mensagens de conversa.

### 12 — P1 — Webhook de cobrança ignora falhas de persistência

**Evidência:** [tratamento do evento](/Users/arturabel/Documents/bi/services/api/internal/billing/stripe.go:103).

O evento e as mudanças de assinatura/plano são gravados com erros ignorados. O método pode retornar sucesso mesmo quando o banco não atualizou o plano. O registro com `ON CONFLICT DO NOTHING` não impede que os efeitos de um evento repetido sejam processados novamente.

**Melhoria:** tratar erros, persistir o evento e os efeitos em transação, registrar estado de processamento e só confirmar recebimento após persistência confiável. Tratar reenvio e ordem dos eventos para que um evento antigo não reverta indevidamente o estado atual.

### 13 — P1 — Infraestrutura local publica bancos em todas as interfaces

**Evidência:** [Docker Compose](/Users/arturabel/Documents/bi/docker-compose.yml:2), [conexão Redis sem autenticação configurável](/Users/arturabel/Documents/bi/services/api/internal/db/db.go:78).

O Compose publica PostgreSQL, Redis, ClickHouse e MinIO sem restringir o endereço a loopback. Há credenciais padrão no arquivo e Redis sem autenticação. O README também descreve o uso desse Compose em VPS.

**Impacto:** se utilizado num host acessível sem bloqueio de rede, serviços internos ficam expostos. A revisão não verificou o firewall nem a implantação real.

**Melhoria:** publicar serviços locais em `127.0.0.1`, ou mantê-los apenas em rede privada entre containers. Separar configuração de desenvolvimento/produção, remover defaults de credenciais em produção e suportar autenticação/TLS nas conexões pertinentes.

### 14 — P1 — Recuperação de senha não revoga sessões anteriores

**Evidência:** [reset de senha](/Users/arturabel/Documents/bi/services/api/internal/authn/extra.go:138), [refresh token](/Users/arturabel/Documents/bi/services/api/internal/authn/service.go:165), [emissão de sessão](/Users/arturabel/Documents/bi/services/api/internal/authn/service.go:255).

O reset altera a senha e marca o token como usado, mas não revoga refresh tokens nem invalida JWTs previamente emitidos. O access token dura oito horas. A renovação usa leitura e revogação separadas, permitindo concorrência entre duas renovações do mesmo token.

**Melhoria:** revogar sessões no reset, usar versão de sessão ou mecanismo de revogação e reduzir duração do access token conforme a experiência desejada. Consumir refresh/reset tokens atomicamente e impedir reutilização. Testar reset durante uma sessão ativa e renovação concorrente.

### 15 — P1 — A suíte de conectores está quebrada

**Evidência:** [teste com servidor local](/Users/arturabel/Documents/bi/services/api/internal/ingest/public_br_test.go:11), [outro exemplo](/Users/arturabel/Documents/bi/services/api/internal/ingest/saas_test.go:10), [cliente HTTP protegido](/Users/arturabel/Documents/bi/services/api/internal/ingest/http.go:19).

Na execução completa houve 15 falhas de ingestão. Os testes usam `httptest.NewServer`, mas os conectores rejeitam IPs internos/loopback. Em um caso, o fallback chegou a consultar uma API pública, mostrando que o teste não está totalmente isolado.

**Melhoria:** injetar cliente/transport HTTP nos conectores para usar respostas controladas em teste. Manter a restrição de rede em produção e testá-la separadamente. Um teste unitário não deve depender de fallback para a internet. Tornar a suíte verde antes de usá-la como critério de entrega.

### 16 — P2 — Gateway compila, mas tem defeitos funcionais reproduzidos

**Evidência:** [driver e cache de conexões](/Users/arturabel/Documents/bi/services/gateway/internal/gateway/gateway.go:104), [limite SQL](/Users/arturabel/Documents/bi/services/gateway/internal/gateway/gateway.go:84), [configuração](/Users/arturabel/Documents/bi/services/gateway/cmd/gateway/main.go:31).

O driver importado para PostgreSQL registra `pgx`, mas `sql.Open` recebe `postgres`/`postgresql`. Isso falha antes de conectar. Consultas que já têm `LIMIT` recebem outro `LIMIT` e ficam inválidas. Ambos foram reproduzidos contra as funções originais. O mapa de conexões é compartilhado entre requisições sem sincronização. O arquivo padrão se chama `gateway.yaml`, mas o parser aceita apenas JSON.

**Melhoria:** mapear tipos aos nomes reais dos drivers, aplicar limite por estratégia segura de cada dialeto, proteger o cache concorrente, definir timeouts/encerramento e alinhar formato de configuração/documentação. Adicionar testes próprios e `-race` ao gateway.

### 17 — P2 — Cache e observabilidade não refletem alterações e uso reais

**Evidência:** [retorno antecipado do cache](/Users/arturabel/Documents/bi/services/api/internal/queryeng/engine.go:103), [chave do cache](/Users/arturabel/Documents/bi/services/api/internal/queryeng/engine.go:936), [registro de histórico](/Users/arturabel/Documents/bi/services/api/internal/queryeng/engine.go:965).

O cache inclui organização, workspace, usuário e consulta, mas não inclui versão do dataset, modelo semântico, papel ou política RLS. O retorno acontece antes da avaliação das regras. Uma mudança pode manter resultado antigo por 30 segundos a dois minutos. Cache hits também retornam antes de registrar histórico, embora a observabilidade consulte `cache_hit` nesse histórico.

**Melhoria:** incluir versões de dados/modelo/permissões na chave ou invalidar explicitamente; assegurar avaliação adequada de acesso antes da reutilização. Registrar cache hits, falhas e latência, distinguindo solicitações do usuário de consultas realmente executadas no ClickHouse.

### 18 — P2 — Limites de volume podem produzir resultados incompletos

**Evidência:** [extração de flow](/Users/arturabel/Documents/bi/services/api/internal/flow/engine.go:84), [exportação](/Users/arturabel/Documents/bi/services/api/internal/apihttp/export.go:34), [lake](/Users/arturabel/Documents/bi/services/api/internal/ingest/engine.go:462), [leitura integral de upload](/Users/arturabel/Documents/bi/services/api/internal/ingest/engine.go:50).

Flows e exportação leem no máximo 100 mil linhas nesse caminho; não há paginação para completar o conjunto. O lake grava CSV limitado a 200 mil linhas no silver e 50 mil no gold, enquanto a arquitetura menciona Parquet. Uploads são lidos integralmente em memória, com limite de leitura de 512 MiB, antes da conversão e geração de outras representações.

**Melhoria:** separar amostra de processamento completo; sinalizar truncamento explicitamente ou rejeitar operações que não possam ser completas. Usar processamento em lotes, exportação assíncrona e controle de memória. Validar tamanho no corpo HTTP e alinhar limites entre proxy, API e interface. Não tratar essas cópias parciais do lake como backup completo.

## Robustez operacional e evolução

| Área | Melhoria recomendada | Evidência/critério |
| --- | --- | --- |
| Migrations | Executar com lock global ou job único; adicionar checksums e procedimento de recuperação | [Migrate](/Users/arturabel/Documents/bi/services/api/internal/db/db.go:113) verifica e aplica sem coordenação entre as réplicas |
| Jobs Python | Consumo com confirmação, retentativa, fila de falhas e tratamento de exceções | [worker](/Users/arturabel/Documents/bi/workers/analytics/main.py:104) remove a tarefa antes de processar e pode encerrar com JSON inválido |
| Agendamentos | Registrar início real e persistir término com contexto independente do timeout do trabalho | [Finish](/Users/arturabel/Documents/bi/services/api/internal/scheduler/store.go:311) ignora erros; o runner reutiliza o contexto que pode estar expirado |
| Encerramento | Cancelar e aguardar loops e trabalhos em andamento antes de fechar conexões | [início dos loops](/Users/arturabel/Documents/bi/services/api/internal/apihttp/server.go:93) usa `context.Background()` |
| Eventos | Definir garantia de entrega, registrar falhas e adotar outbox se os eventos forem parte da consistência do produto | [bus](/Users/arturabel/Documents/bi/services/api/internal/events/bus.go:45) pode iniciar em modo que descarta eventos; writers são assíncronos |
| Entregas | Usar versões/digests imutáveis, rollback e restauração de backup verificada | Manifests usam `latest` com `IfNotPresent`; Terraform é um esqueleto, não provisionamento completo |
| CI | Alinhar Go do CI/Dockerfile ao módulo; usar `npm ci`, build, testes de UI/API, gateway e worker | [.github/workflows/ci.yml](/Users/arturabel/Documents/bi/.github/workflows/ci.yml:1) usa Go 1.24 e só checa tipos no web; o módulo declara Go 1.25 |
| Contratos | Completar OpenAPI e gerar tipos de entrada/saída para o cliente | [OpenAPI](/Users/arturabel/Documents/bi/docs/openapi.yaml:1) tem apenas 9 caminhos; frontend usa muitos `any` |
| Auditoria | Registrar falhas de gravação e evitar respostas de sucesso depois de persistência obrigatória falhar | Há vários `_ =`/`_, _ =` nos caminhos de auditoria, cobrança, jobs e metadados |

## Frontend, experiência e manutenção

1. **Dividir os componentes centrais por responsabilidade.** O inspetor tem 1.436 linhas, o editor de dashboard 1.171 e `WidgetView` 984. Separar estado do editor, persistência, filtros, consultas e renderizadores. Consolidar catálogo/configuração de widgets compartilhados entre dashboards e relatórios, com tipos explícitos.

2. **Criar testes de comportamento na interface.** Não há arquivos de teste frontend versionados nem scripts de teste/lint. Priorizar login/MFA, convite existente, troca de workspace, upload → consulta → dashboard, viewer, compartilhamento, filtros e exportação. Adicionar testes de integração com dois tenants e usuários de papéis diferentes.

3. **Tornar permissões e erros compreensíveis.** O editor de dados decide se pode editar pelo tipo de fonte, sem considerar o papel ([editor](/Users/arturabel/Documents/bi/apps/web/src/components/dataset-data-editor.tsx:64)), enquanto a API exige admin. Ocultar/desabilitar ações conforme permissões reais e explicar a restrição. Na troca de workspace, mostrar falha ao usuário em vez de somente `console.error`.

4. **Padronizar modais acessíveis.** O modal de medida é um overlay de `div` sem contrato completo de diálogo ([modal](/Users/arturabel/Documents/bi/apps/web/src/components/custom-measure-modal.tsx:201)). Adicionar nome acessível, contenção e restauração do foco, navegação por teclado e fechamento apropriado. Validar leitor de tela e teclado em execução; esta revisão não mediu acessibilidade visual.

5. **Medir e reduzir carregamento de telas analíticas.** O build mediu aproximadamente 353 kB de JavaScript inicial no editor de dashboard, 325 kB em relatórios e 311–312 kB em embed/share. Isso orienta investigação, não comprova lentidão. Separar renderizadores por demanda, evitar importar gráficos em telas que só exibem indicadores simples e medir experiência com painéis reais.

6. **Explicitar a confiabilidade dos dados.** Mostrar última atualização bem-sucedida, processamento em andamento, resultado parcial, quantidade total versus retornada, origem e versão da métrica. Diferenciar “sem dados”, erro e resultado zero. Não apresentar um flow como concluído se a saída falhou.

7. **Fortalecer confiança e controle de custos da IA.** Manter validação semântica; adicionar avaliações com perguntas e números esperados, dados incompletos e ambiguidades. Medir latência, consumo, taxa de respostas recusadas e qualidade. Definir política de envio de amostras e campos sensíveis ao provedor, limites por organização e falhas visíveis ao usuário.

8. **Revisar documentação e artefatos versionados.** O README aponta porta 3000, o script de desenvolvimento usa 3010, e os defaults do Compose diferem dos apresentados no texto. Há três CSVs operacionais na raiz com colunas de cliente/telefone: verificar anonimização e necessidade de versionamento. `cookies.txt` está versionado, mas estava sem entradas de cookies nesta revisão; não foi constatado vazamento por esse arquivo. Remover do versionamento artefatos gerados como `tsconfig.tsbuildinfo` e o arquivo vazio em `src/components/WidgetView.tsx`, após verificar uso.

## Sequência recomendada

| Etapa | Entrega | Critério para avançar |
| --- | --- | --- |
| 1 — Segurança | Corrigir convites, SAML, XSS, isolamento de flows, RLS e matriz de permissões | Testes demonstram que A não lê/altera B; viewer não publica/edita; convites não autenticam terceiros; conteúdo hostil é inerte |
| 2 — Integridade | Substituição atômica, CDC idempotente e estados de execução verdadeiros | Falha no meio da carga preserva a versão anterior; reprocessamento não duplica; timestamps iguais não perdem registros |
| 3 — Entrega confiável | Corrigir os 15 testes, ampliar integração, validar containers/migrations e alinhar catálogo/cobrança | CI executa a jornada crítica e falhas de persistência são observáveis e recuperáveis |
| 4 — Operação e UX | Jobs recuperáveis, exportação completa, cache coerente, acessibilidade e redução medida do carregamento | Recuperação testada após reinício; volumes completos ou truncamento explícito; permissões e atualização compreensíveis na UI |

Não começaria por uma reescrita de arquitetura ou por novos conectores. O melhor retorno está em tornar os recursos existentes seguros, corretos e verificáveis, preservando a organização modular atual.
