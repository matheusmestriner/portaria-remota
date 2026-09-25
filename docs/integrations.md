# Contratos e integrações

## API

Base: `/v1`. Usuários autenticados enviam `Authorization: Bearer <access_token>` e `X-Company-Id: <UUID>` nas operações de empresa. O servidor valida assinatura RS256, emissor, audiência `portaria-api`, segundo fator e vínculos atuais. Os privilégios de negócio vêm do banco, não de campos enviados pelo navegador.

| Operação | Rota | Dados principais |
|---|---|---|
| Criar empresa | `POST /companies` | `name`, `slug` |
| Personalizar | `PATCH /companies/:id/brand` | `name`, `primary`, `logo?` HTTPS |
| Abrir suporte | `POST /companies/:id/support` | `reason`, mínimo 10 caracteres |
| Criar condomínio | `POST /condos` | `name`, `address` |
| Vincular usuário | `POST /memberships` | `subject`, `name`, `role`, `condo_ids`, `unit_ids` |
| Criar cadastro | `POST /records/:kind` | `condo_id`, `unit_id?`, `data` validado pelo tipo |
| Criar convite | `POST /invitations` | `unit_id`, `guest_name`, `valid_from`, `valid_until`, `access_ids`, `request_key`, `channel`, `vehicle_plate?` |
| Cancelar convite | `POST /invitations/:id/cancel` | Objeto vazio |
| Solicitar abertura | `POST /commands` | `access_id`, `reason`, `request_key`, `authorization_id?` |
| Credenciais conferidas | `GET /access-authorizations` | Somente autorizações pendentes dentro do escopo |
| Assumir atendimento | `POST /tickets/:id/claim` | Atualização atômica de responsável |
| Encerrar atendimento | `POST /tickets/:id/close` | Somente o responsável |
| Histórico | `GET /events` | Até 100 registros no escopo |
| Atualização contínua | `GET /events/stream` | SSE com autorização revalidada |

Datas em ISO 8601 com fuso. Convites duram no máximo sete dias e permitem uma visita. A página pública usa um token aleatório de 256 bits; somente seu hash fica indexado no banco. A senha temporária fica cifrada com AES-256-GCM. Os erros usam `message`; validações podem incluir `details`.

Listagens administrativas têm limite de 500 registros nesta versão. Paginação por cursor, relatórios analíticos completos e convites recorrentes ainda precisam de evolução para operações grandes. O campo de regra `allow_recurring` não habilita recorrência na emissão atual.

## Agente e equipamentos

Cada condomínio recebe um token exclusivo. A API armazena somente seu hash. Revogar um agente interrompe suas próximas requisições. Configure o agente com `services/agent/config.example.json`, guarde os segredos fora do repositório e monte um volume durável em `/var/lib/portaria`.

O agente faz chamadas de saída para a API por HTTPS. Sua interface local fica em loopback por padrão. Se for necessário recebimento direto de equipamento na LAN, configure uma interface privada específica, autenticação e firewall; não publique essa interface na internet.

Mapeie cada UUID real de dispositivo em `devices`, com URL e token do adaptador. A URL vem da configuração local, nunca de um comando de abertura recebido da nuvem.

### Contrato `http-relay-v1`

`POST /open` no adaptador recebe `command_id`, `access_id` e `expires_at`, com `Authorization: Bearer <segredo local>` e `Idempotency-Key`. Só retorne `{ "confirmed": true, "command_id": "..." }` quando houver confirmação real do equipamento. O adaptador também deve rejeitar comandos vencidos e repetidos. HTTP 200 isolado não confirma a operação.

`PUT /credentials/:id` recebe `credential_id`, `type`, `reference` e `desired_state: active|revoked`. A operação deve ser idempotente. Retorne `credential_id` e `state` somente após confirmação do equipamento. A plataforma mantém `pending` ou `revocation_pending` até receber esse retorno.

O reconhecimento facial e de placas ocorre no equipamento. Esta versão trabalha com referências já cadastradas nele; captura e envio de fotografias faciais dependem do adaptador específico. Nenhuma fotografia biométrica é aceita ou armazenada pelo código atual.

O leitor envia a credencial temporária para `POST /credentials/validate` no agente, com `credential`, `access_id`, `request_key` e autenticação local. O agente consulta a API em tempo real. No modo automático, uma autorização válida pode produzir uma abertura; no modo combinado, gera uma pendência para conferência do operador. Sem conexão, convidados não são liberados por este fluxo.

Eventos físicos chegam por `POST /events` no agente: `access_id`, `kind` e `source_key` estável. Os tipos aceitos incluem `passage.detected`, `door.open`, `door.closed` e `device.offline`. O equipamento/adaptador deve manter eventos não confirmados e repetir com a mesma chave. O agente não inventa passagem a partir de abertura e não tem uma fila própria durável de eventos físicos nesta versão.

Comandos remotos expiram em dez segundos. O agente registra a intenção em disco antes de acionar o dispositivo. Após falha ambígua ou reinício, não repete o comando. Uma confirmação perdida permanece desconhecida para a plataforma; não se converte em sucesso presumido.

### Homologação

O relatório JSON deve conter `manufacturer`, `model`, `firmware`, `operator`, `performed_at`, `adapter: http-relay-v1`, `evidence`, `capabilities` e `tests`. Devem estar aprovados: `no_replay`, `expiry`, `unknown_result`, `network_failure`, `physical_confirmation`, `revocation`, `tenant_isolation`. Isso registra uma evidência informada pelo responsável; o script não realiza testes físicos sozinho.

## Whatsmeow

Execute um gateway Whatsmeow por revenda/tenant, com banco ou schema exclusivo e usuário SQL restrito àquela revenda. Não utilize a conexão administrativa da plataforma. Um gateway pode manter várias sessões simultâneas: uma conta padrão da revenda e uma conta independente para cada condomínio. Configure `WHATSAPP_BRIDGES` na API como um mapa JSON de UUID real da revenda para a URL interna do gateway.

Abra **Integrações → WhatsApp → Configurar**. O administrador pode escolher **QR Code** ou **Código de pareamento**. No modo QR, o código visual aparece somente para administradores autorizados. No modo código, informe o número em formato E.164, por exemplo `+5511999999999`; a ponte conecta ao WhatsApp, espera o handshake inicial e chama `PairPhone`, retornando um código temporário. O código deve ser informado no WhatsApp do número escolhido antes de expirar. Os dados de sessão persistem no SQL store do Whatsmeow. O código de pareamento é efêmero e não é persistido no banco da plataforma.

O morador gera um código no app/web e envia `VINCULAR <código>` ao número da portaria. O código expira em dez minutos. Depois envia `CONVITE`, escolhe unidade, informa convidado, início/fim, placa opcional, acesso e confirma. A conversa expira após quinze minutos de inatividade e aceita `CANCELAR`.

O bridge ignora grupos, mensagens enviadas pelo próprio número, edições e mensagens antigas. Identificadores LID são resolvidos pelo armazenamento do Whatsmeow antes de procurar o número vinculado. Falha na resolução não autoriza acesso. Mensagens são deduplicadas no banco, e respostas com links ficam cifradas.

Após o pareamento, `GET /status` da ponte retorna o número autenticado em E.164 e a API persiste esse número como metadado não sensível da integração. O painel mostra qual número da portaria está conectado.

A ponte também aceita envio ativo em `POST /send`, autenticado por `X-Service-Key`. Corpo: `to` em E.164, `text` e `request_key`. O `request_key` é persistido no banco exclusivo da ponte para impedir reenvio silencioso da mesma mensagem. Se houver falha ambígua depois da tentativa de envio, a chave fica em estado incerto e não é reenviada automaticamente com a mesma chave.

A API expõe `POST /v1/whatsapp/send` para administradores/supervisores e `POST /v1/internal/whatsapp/send` para serviços internos autenticados. Isso é a base para notificações transacionais, incluindo chegada e retirada de entregas. Não usar esse canal para disparos promocionais em massa.

O bridge mantém uma fila em memória para mensagens recebidas. Uma interrupção pode exigir nova interação do morador; consulte a lista de convites antes de reenviar uma confirmação. Aprovação na chegada e comando de abertura pelo WhatsApp continuam desabilitados.


### Hierarquia de contas

A tela **Integrações → WhatsApp** lista `Número padrão da revenda` e todos os condomínios acessíveis ao administrador. Cada item pode ser pareado separadamente por QR Code ou código de pareamento. O banco da plataforma registra a conta em `whatsapp_accounts`, sempre com `company_id` e, quando específica, `condo_id`.

Mensagens recebidas carregam o identificador interno da sessão. `whatsapp_messages` e `whatsapp_flows` usam `account_id`, portanto o mesmo morador pode conversar com dois números da revenda sem compartilhar o estado do bot entre as conversas. Em uma conta de condomínio, o bot só aceita moradores e unidades daquele condomínio.

Para mensagens transacionais, como entrega recebida ou retirada, a API resolve a conta nesta ordem:

1. conta conectada do condomínio;
2. conta padrão conectada da revenda;
3. se nenhuma estiver conectada, o envio falha de forma explícita e pode ser reprocessado pela fila.

A revenda continua isolada das demais no nível da aplicação e também no gateway: cada revenda possui processo e armazenamento Whatsmeow próprios. Contas de condomínios diferentes compartilham apenas o gateway da própria revenda, nunca o gateway de outra revenda.

## Câmeras

Use caminhos `<UUID da empresa>/<stream_path>` no MediaMTX. Configure somente fontes RTSP reais acessíveis por rede privada/VPN. A API emite tokens de leitura com caminho, empresa e usuário, válidos por sessenta segundos. O callback de autenticação revalida vínculo ou suporte e situação da empresa. O player renova a sessão a cada cinquenta segundos.

A revogação durante uma transmissão já aberta depende da renovação/desconexão do player; MediaMTX não é consultado continuamente pela API. Para revogação imediata em produção, integre encerramento ativo da sessão no servidor de mídia.

Configure `MEDIA_PUBLIC_URL` com HTTPS. Exponha somente WHEP/WebRTC pelo proxy e as portas ICE necessárias; mantenha RTSP, métricas e interfaces administrativas privados. Compatibilidade de codec e NAT/TURN precisa de testes na rede real. Não há gravação contínua nesta versão.

## Interfonia

O painel usa SIP.js sobre WSS. O operador conecta sua conta, atende, recusa e encerra chamadas. Credenciais SIP ficam cifradas no banco e são entregues somente ao operador correspondente depois da autenticação.

Cadastre uma conta real pelo endpoint `POST /telephony/accounts`, com `subject`, `username` e senha forte correspondentes ao Asterisk. O endpoint não cria ramais no Asterisk: provisione o servidor com os templates de `infra/asterisk`, separando contextos por empresa e rotas por condomínio. Não permita discagem entre empresas nem troncos externos implícitos.

O servidor de telefonia deve enviar `ringing`, `answered`, `ended`, `missed` para `POST /internal/telephony/events`, autenticado com `X-Service-Key`. Corpo: `company_id`, `condo_id`, `access_id`, `call_id`, `kind`. Repetições são deduplicadas. Chamadas recebidas criam atendimento; chamadas perdidas geram ocorrência.

O serviço Go em services/telephony recebe UserEvents PortariaCall por AMI, aplica um mapa local ACCESS_BINDINGS e mantém eventos em disco até a confirmação da API. Configure uma conta AMI somente de leitura, TLS e o mapa dos acessos reais. O contexto portaria-notify mostra a emissão dos eventos; integre-o ao dialplan do condomínio. Esse serviço não foi compilado ou conectado a um PABX nesta máquina. Os templates não criam ramais fictícios.
