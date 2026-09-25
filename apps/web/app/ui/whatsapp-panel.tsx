'use client';
import { useCallback,useEffect,useMemo,useState } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import { MessageCircle,RefreshCw,Link2,Send,Unplug,ScanLine,Hash } from 'lucide-react';

type PairMode='qr'|'code';
type AccountSlot={condo_id:string|null;bridge_key:string;label:string;account_id:string|null;status:string;phone?:string|null;configured:boolean};

export default function WhatsAppPanel({api}:{api:(path:string,body?:any)=>Promise<any>}){
 const[accounts,setAccounts]=useState<AccountSlot[]>([]);
 const[selected,setSelected]=useState('default');
 const[state,setState]=useState<any>();
 const[error,setError]=useState('');
 const[notice,setNotice]=useState('');
 const[busy,setBusy]=useState(false);
 const[to,setTo]=useState('');
 const[text,setText]=useState('Teste de integração PortalIA.');
 const[pairMode,setPairMode]=useState<PairMode>('qr');
 const[pairPhone,setPairPhone]=useState('');

 const slot=useMemo(()=>accounts.find(a=>a.bridge_key===selected),[accounts,selected]);
 const condoId=slot?.condo_id||undefined;
 const query=condoId?`?condo_id=${encodeURIComponent(condoId)}`:'';

 const loadAccounts=useCallback(async()=>{try{const list=await api('whatsapp/accounts');setAccounts(list);if(list.length&&!list.some((a:AccountSlot)=>a.bridge_key===selected))setSelected(list[0].bridge_key);}catch(e){setError((e as Error).message);}},[api,selected]);

 const refresh=useCallback(async()=>{try{const next=await api(`whatsapp/session${query}`);setState(next);setError('');}catch(e){setError((e as Error).message);}},[api,query]);

 useEffect(()=>{void loadAccounts();},[loadAccounts]);
 useEffect(()=>{setState(undefined);setNotice('');setError('');void refresh();const id=setInterval(refresh,4000);return()=>clearInterval(id);},[refresh]);

 async function connect(){setBusy(true);setNotice('');try{setState(await api('whatsapp/connect',{...(condoId?{condo_id:condoId}:{})}));setError('');}catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 async function pairByCode(){setBusy(true);setNotice('');try{setState(await api('whatsapp/pair-code',{...(condoId?{condo_id:condoId}:{}),phone:pairPhone}));setError('');}catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 async function disconnect(){setBusy(true);setNotice('');try{setState(await api('whatsapp/disconnect',{...(condoId?{condo_id:condoId}:{})}));setNotice('WhatsApp desconectado. A sessão pareada foi preservada.');setError('');await loadAccounts();}catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 async function sendTest(){setBusy(true);setNotice('');try{await api('whatsapp/send',{...(condoId?{condo_id:condoId}:{}),to,text,request_key:`admin-test:${crypto.randomUUID()}`});setNotice('Mensagem de teste enviada pelo número selecionado.');setError('');}catch(e){setError((e as Error).message);}finally{setBusy(false);}}

 const connected=state?.status==='connected';
 const pairingCode=state?.pair_code;

 return <><span className="modal-symbol"><MessageCircle size={26}/></span><h2>WhatsApp</h2><p>Configure um número padrão para a revenda ou um número exclusivo para cada condomínio.</p>

 <div className="field"><span>Conta</span><select value={selected} onChange={e=>setSelected(e.target.value)}>{accounts.map(a=><option key={a.bridge_key} value={a.bridge_key}>{a.condo_id?a.label:`${a.label} · padrão`}{a.phone?` · ${a.phone}`:''}</option>)}</select></div>
 {slot?.condo_id?<p className="form-note">Mensagens deste número ficam restritas a {slot.label}. Se este número não estiver conectado, notificações automáticas podem usar o número padrão da revenda.</p>:<p className="form-note">Este número funciona como fallback para condomínios que não possuem uma conta própria conectada.</p>}

 {connected?<div className="alert success"><Link2 size={18}/><span>WhatsApp conectado{state.phone?` · ${state.phone}`:''}</span></div>:<>
  <div className="pair-methods">
   <button type="button" className={`btn ${pairMode==='qr'?'primary':'secondary'}`} onClick={()=>setPairMode('qr')}><ScanLine size={16}/> QR Code</button>
   <button type="button" className={`btn ${pairMode==='code'?'primary':'secondary'}`} onClick={()=>setPairMode('code')}><Hash size={16}/> Código de pareamento</button>
  </div>

  {pairMode==='qr'?<>
   {state?.qr?<div className="qr-box"><QRCodeSVG value={state.qr} size={230}/></div>:<div className="small-empty"><ScanLine size={32}/><p>Gere o QR Code para conectar o número selecionado.</p></div>}
   {state?.qr?<p className="form-note">No celular desse número, abra WhatsApp, Aparelhos conectados e leia este QR Code.</p>:<button className="btn primary" disabled={busy} onClick={()=>void connect()}>Gerar QR Code</button>}
  </>:<>
   {pairingCode?<div className="card"><h3>Código de pareamento</h3><strong className="pair-code">{pairingCode}</strong><p className="form-note">Número: {state?.pair_phone||pairPhone}</p><p className="form-note">No WhatsApp do número informado, abra Aparelhos conectados, escolha a vinculação por código e informe o código acima antes que expire.</p></div>:<div className="card"><h3>Conectar por número</h3><div className="field"><span>Número com DDI</span><input inputMode="tel" value={pairPhone} onChange={e=>setPairPhone(e.target.value.replace(/\s/g,''))} placeholder="+5511999999999"/></div><button className="btn primary" disabled={busy||!/^\+[1-9]\d{7,14}$/.test(pairPhone)} onClick={()=>void pairByCode()}><Hash size={16}/> Gerar código</button></div>}
  </>}
 </>}

 {connected&&<div className="card"><h3>Testar envio</h3><div className="field"><span>Destino</span><input value={to} onChange={e=>setTo(e.target.value)} placeholder="+5511999999999"/></div><div className="field"><span>Mensagem</span><input value={text} onChange={e=>setText(e.target.value)} maxLength={2000}/></div><button className="btn primary" disabled={busy||!/^\+[1-9]\d{7,14}$/.test(to)||!text.trim()} onClick={()=>void sendTest()}><Send size={16}/> Enviar mensagem de teste</button></div>}
 {notice&&<div className="alert success">{notice}</div>}{error&&<p className="field-error">{error}</p>}
 <div className="modal-actions"><button className="btn secondary" onClick={()=>{void loadAccounts();void refresh();}}><RefreshCw size={16}/> Atualizar</button>{connected&&<button className="btn secondary" disabled={busy} onClick={()=>void disconnect()}><Unplug size={16}/> Desconectar</button>}</div></>;
}
