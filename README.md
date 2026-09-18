# Vértice — plataforma de portaria remota

Implementação de uma plataforma com clientes white label, painel web/PWA, API NestJS, PostgreSQL com RLS, aplicativo Expo, integração Whatsmeow e agente local Go.

**A instalação começa vazia.** Nenhuma empresa, condomínio, morador ou movimentação de demonstração é criada. O único cadastro inicial é o administrador real criado pelo procedimento de bootstrap.

## Estado da entrega

O painel e a API compilam. Os testes do núcleo executam PostgreSQL em memória através do PGlite, com regras RLS reais. O projeto inclui código dos aplicativos e conectores, mas **a operação comercial não foi homologada**.

| Área | Entrega | Validação nesta máquina |
|---|---|---|
| Painel web e PWA | Navegação, cadastros, white label, convites, central e integrações | TypeScript e build de produção aprovados; inspeção visual não autorizada |
| API e banco | Autorização, RLS, convites, comandos, auditoria, atendimentos e integrações | 22 testes aprovados em PostgreSQL/WASM; sem servidor PostgreSQL externo |
| Autenticação | Configuração Keycloak, MFA obrigatório, bootstrap e vínculos por perfil | Configuração fornecida; Keycloak não executado nesta máquina |
| WhatsApp | Conversa de convite, vinculação de número, deduplicação e ponte Go Whatsmeow | Fluxo de negócio testado; ponte Go não compilada nem pareada |
| Android e iPhone | App Expo com login PKCE, convites, compartilhamento e cancelamento | Código entregue; dependências nativas, emuladores e builds não executados |
| Agente local | Expiração, diário persistente, execução sem repetição, eventos e sincronização | Código e testes Go entregues; Go indisponível nesta máquina |
| Vídeo e interfonia | WHEP/WebRTC, SIP.js, autorização de mídia e eventos de chamadas | TypeScript aprovado; sem câmeras/interfones para testar |
| Deploy e recuperação | Contêineres, migrações, worker, proxy, backup e restauração | Scripts TypeScript compilados; Docker e recuperação externa não executados |

Os equipamentos não têm fabricante/modelo definido. O conector `http-relay-v1` é um **contrato para o adaptador local**, não uma alegação de compatibilidade nativa com Control iD, Intelbras ou outro fabricante. Reconhecimento facial e leitura de placas dependem do reconhecimento feito por equipamento homologado. O aplicativo não armazena fotografias biométricas.

## Abrir o painel local

Requisitos: Node.js 24 e npm. Na pasta deste projeto:

```powershell
npm ci
npm run dev
```

Abra `http://localhost:3000`. Sem infraestrutura configurada, o painel apresenta orientações e estados vazios. Não existe login de demonstração nem modo que desative a autenticação.

## Configurar a plataforma

Requisitos adicionais: Docker com Compose, SMTP para enviar a ativação da conta, domínio sob seu controle e certificados válidos para publicação. Os comandos abaixo são para ambiente local.

```powershell
npm run env:init
docker compose up -d postgres redis keycloak
npm run db:migrate
npm run bootstrap
```

`env:init` cria `.env` com segredos aleatórios e recusa sobrescrever um arquivo existente. `db:migrate` cria tabelas, políticas e o login PostgreSQL restrito usado pela API.

O bootstrap solicita seu e-mail real e cadastra exclusivamente sua conta. No Keycloak, configure SMTP e envie ao usuário as ações **verificar e-mail, definir senha e configurar TOTP**. Complete o fluxo recebido por e-mail. As credenciais administrativas da infraestrutura ficam em `.env`; não estão no código.

O assistente **Prepare sua plataforma** também verifica banco, autenticação e tarefas e permite criar o primeiro administrador pelo painel. Em produção, essa ação exige o `SETUP_TOKEN`, funciona somente enquanto não existe administrador e exige temporariamente `MIGRATION_DATABASE_URL` e as credenciais administrativas do Keycloak no serviço da API. Remova `MIGRATION_DATABASE_URL`, `KC_BOOTSTRAP_ADMIN_USERNAME`, `KC_BOOTSTRAP_ADMIN_PASSWORD` e `SETUP_TOKEN` do serviço após concluir o primeiro acesso. O comando `npm run bootstrap` continua disponível como alternativa operacional.

Depois, inicie em terminais separados:

```powershell
npm run dev:api
npm run worker
npm run dev
```

O comando de desenvolvimento da API compila antes de iniciar. Após editar o código da API, reinicie-o para recompilar. A documentação das rotas fica em `http://localhost:4000/docs`.

Para executar os aplicativos em contêineres após as migrações:

```powershell
docker compose -f compose.yaml -f compose.application.yaml up --build api web worker
```

O Compose base utiliza `start-dev` e armazenamento local do Keycloak: **é uma configuração local**, não uma configuração final de produção. A publicação exige o procedimento em [docs/deployment.md](docs/deployment.md).

## Cadastrar seus clientes

1. Entre na plataforma e clique em **Novo cliente**.
2. Cadastre a empresa real. O estado inicial é “Em configuração”.
3. Ao acessar sua operação como administrador da plataforma, informe o motivo do suporte. O acesso é auditado e dura 30 minutos.
4. Configure nome e logotipo HTTPS em **White label**. A interface permanece em preto e branco.
5. Adicione o domínio e os registros TXT/CNAME apresentados. O worker verifica HTTPS usando o domínio e o destino configurados.
6. Após DNS e HTTPS válidos, execute `npm run domain:authorize -- portaria.cliente.com.br`. Isso autoriza somente esse domínio no login do Keycloak.
7. Crie o usuário administrador da empresa no Keycloak e vincule seu identificador em **Equipe e permissões**.
8. Cadastre condomínios, unidades, pessoas, equipamentos, pontos de acesso e moradores. Operadores e moradores precisam das atribuições correspondentes.
9. Configure o agente local, realize os testes de bancada e registre o relatório técnico com `npm run homologate -- UUID_DO_EQUIPAMENTO caminho/relatorio.json`.
10. Ative o cliente após concluir as etapas exibidas no painel.

A ativação não dispensa a homologação de cada equipamento utilizado. A API bloqueia convites e aberturas em dispositivos não homologados ou condomínios sem conexão.

## Aplicativo Android/iPhone

O projeto nativo tem dependências separadas do painel para evitar conflitos entre React DOM e React Native:

```powershell
cd apps/mobile
npm install
npx expo install --fix
```

Copie `.env.example` para `.env` e preencha os endereços HTTPS reais de API e autenticação. Execute `npx expo start`. O fluxo OAuth deve ser testado em um development build com o esquema `portaria://auth`, não presumido como funcional no Expo Go.

Antes de publicar, configure um projeto EAS, substitua os identificadores `br.com.vertice.portaria` por identificadores sob seu controle e use suas contas Apple/Google. Os aplicativos usam a marca da plataforma; o white label é restrito à web/PWA.

## Testes

```powershell
npm test
npm run typecheck
npm run build
```

Os testes criam fixtures exclusivamente em um banco efêmero em memória. Não acessam os dados de produção.

Com Go disponível:

```powershell
cd services/agent
go test ./...
go build ./...
```

A ponte Whatsmeow tem seu próprio Dockerfile e requer acesso ao registro de módulos Go. A resolução de dependências e geração de `go.sum` precisam ser executadas e versionadas durante a validação da integração. Consulte [docs/integrations.md](docs/integrations.md).

## Estrutura

```text
apps/web          Painel Next.js, PWA e página pública de convite
apps/api          API NestJS, regras, segurança e worker BullMQ
apps/mobile       Aplicativo Expo Android/iPhone
database          Migrações PostgreSQL, RLS e funções restritas
services/agent    Agente Go e contrato do adaptador local
services/whatsmeow Ponte Go por empresa
infra             Keycloak, MediaMTX, Asterisk, proxy e contêineres
scripts           Bootstrap, migrações, homologação e recuperação
tests             Regressões de autorização e operação
docs              Contratos, implantação e validação
```

O nome Vértice é uma identidade inicial substituível. O projeto não inclui registro de marca, hospedagem contratada, domínio, publicação em lojas nem equipamentos físicos.

