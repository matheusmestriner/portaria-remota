'use client';
import { useEffect,useState } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import { MessageCircle,RefreshCw,Link2,Send,Unplug,ScanLine,Hash } from 'lucide-react';

type PairMode='qr'|'code';

export default function WhatsAppPanel({api}:{api:(path:string,body?:any)=>Promise<any>}){
 const[state,setState]=useState<any>(),[error,setError]=useState(''),[notice,setNotice]=useState(''),[busy,setBusy]=useState(false),[to,setTo]=useState(''),[text,setText]=useState('Teste de integração PortalIA.'),[pairMode,setPairMode]=useState<PairMode>('qr'),[pairPhone,setPairPhone]=useState('');
 async function refresh(){try{setState(await api('whatsapp/session'));setError('');}catch(e){setError((e as Error).message);}}
 useEffect(()=>{void refresh();const id=setInterval(refresh,4000);return()=>clearInterval(id);},[api]);
 async function connect(){setBusy(true);setNotice('');try{setState(await api('whatsapp/connect',{}));setError('');}catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 async function pairByCode(){setBusy(true);setNotice('');try{setState(await api('whatsapp/pair-code',{phone:pairPhone}));setError('');}catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 async function disconnect(){setBusy(true);setNotice('');try{setState(await api('whatsapp/disconnect',{}));setNotice('WhatsApp desconectado. A sessão pareada foi preservada.');setError('');}catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 async function sendTest(){setBusy(true);setNotice('');try{await api('whatsapp/send',{to,text,request_key:`admin-test:${crypto.randomUUID()}`});setNotice('Mensagem de teste enviada.');setError('');}catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 const connected=state?.status==='connected';
 const pairingCode=state?.pair_code;
 return <><span className="modal-symbol"><MessageCircle size={26}/></span><h2>WhatsApp da portaria</h2><p>Conecte o número da portaria por QR Code ou por código de pareamento. A sessão fica isolada para esta empresa.</p>

 {connected?<div className="alert success"><Link2 size={18}/><span>WhatsApp conectado{state.phone?` · ${state.phone}`:''}</span></div>:<>
  <div className="pair-methods">
   <button type="button" className={`btn ${pairMode==='qr'?'primary':'secondary'}`} onClick={()=>setPairMode('qr')}><ScanLine size={16}/> QR Code</button>
   <button type="button" className={`btn ${pairMode==='code'?'primary':'secondary'}`} onClick={()=>setPairMode('code')}><Hash size={16}/> Código de pareamento</button>
  </div>

  {pairMode==='qr'?<>
   {state?.qr?<div className="qr-box"><QRCodeSVG value={state.qr} size={230}/></div>:<div className="small-empty"><ScanLine size={32}/><p>Gere o QR Code para conectar o WhatsApp da portaria.</p></div>}
   {state?.qr?<p className="form-note">No celular da portaria, abra WhatsApp, Aparelhos conectados e leia este QR Code.</p>:<button className="btn primary" disabled={busy} onClick={()=>void connect()}>Gerar QR Code</button>}
  </>:<>
   {pairingCode?<div className="card"><h3>Código de pareamento</h3><strong className="pair-code">{pairingCode}</strong><p className="form-note">Número: {state?.pair_phone||pairPhone}</p><p className="form-note">No WhatsApp do número informado, abra Aparelhos conectados e escolha a opção de vincular usando um número ou código. Informe o código acima antes que ele expire.</p></div>:<div className="card"><h3>Conectar por número</h3><div className="field"><span>Número com DDI</span><input inputMode="tel" value={pairPhone} onChange={e=>setPairPhone(e.target.value.replace(/\s/g,''))} placeholder="+5511999999999"/></div><button className="btn primary" disabled={busy||!/^\+[1-9]\d{7,14}$/.test(pairPhone)} onClick={()=>void pairByCode()}><Hash size={16}/> Gerar código</button></div>}
  </>}
 </>}

 {connected&&<div className="card"><h3>Testar envio</h3><div className="field"><span>Destino</span><input value={to} onChange={e=>setTo(e.target.value)} placeholder="+5511999999999"/></div><div className="field"><span>Mensagem</span><input value={text} onChange={e=>setText(e.target.value)} maxLength={2000}/></div><button className="btn primary" disabled={busy||!/^\+[1-9]\d{7,14}$/.test(to)||!text.trim()} onClick={()=>void sendTest()}><Send size={16}/> Enviar mensagem de teste</button></div>}
 {notice&&<div className="alert success">{notice}</div>}{error&&<p className="field-error">{error}</p>}
 <div className="modal-actions"><button className="btn secondary" onClick={()=>void refresh()}><RefreshCw size={16}/> Atualizar</button>{connected&&<button className="btn secondary" disabled={busy} onClick={()=>void disconnect()}><Unplug size={16}/> Desconectar</button>}</div></>;
}
