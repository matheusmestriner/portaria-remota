'use client';
import { useEffect,useState } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import { MessageCircle,RefreshCw,Link2,Send,Unplug } from 'lucide-react';

export default function WhatsAppPanel({api}:{api:(path:string,body?:any)=>Promise<any>}){
 const[state,setState]=useState<any>(),[error,setError]=useState(''),[notice,setNotice]=useState(''),[busy,setBusy]=useState(false),[to,setTo]=useState(''),[text,setText]=useState('Teste de integração PortalIA.');
 async function refresh(){try{setState(await api('whatsapp/session'));setError('');}catch(e){setError((e as Error).message);}}
 useEffect(()=>{void refresh();const id=setInterval(refresh,4000);return()=>clearInterval(id);},[api]);
 async function connect(){setBusy(true);setNotice('');try{setState(await api('whatsapp/connect',{}));setError('');}catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 async function disconnect(){setBusy(true);setNotice('');try{setState(await api('whatsapp/disconnect',{}));setNotice('WhatsApp desconectado. A sessão pareada foi preservada.');setError('');}catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 async function sendTest(){setBusy(true);setNotice('');try{await api('whatsapp/send',{to,text,request_key:`admin-test:${crypto.randomUUID()}`});setNotice('Mensagem de teste enviada.');setError('');}catch(e){setError((e as Error).message);}finally{setBusy(false);}}
 return <><span className="modal-symbol"><MessageCircle size={26}/></span><h2>WhatsApp da portaria</h2><p>Vincule o número usado pela portaria. A sessão fica isolada para esta empresa e o número pareado é identificado automaticamente pelo Whatsmeow.</p>
 {state?.status==='connected'?<div className="alert success"><Link2 size={18}/><span>WhatsApp conectado{state.phone?` · ${state.phone}`:''}</span></div>:state?.qr?<div className="qr-box"><QRCodeSVG value={state.qr} size={230}/></div>:<div className="small-empty"><MessageCircle size={32}/><p>Aguardando pareamento do número da portaria.</p></div>}
 {state?.qr&&<p className="form-note">No celular da portaria, abra WhatsApp, Aparelhos conectados e leia este QR Code.</p>}
 {state?.status==='connected'&&<div className="card"><h3>Testar envio</h3><div className="field"><span>Destino</span><input value={to} onChange={e=>setTo(e.target.value)} placeholder="+5511999999999"/></div><div className="field"><span>Mensagem</span><input value={text} onChange={e=>setText(e.target.value)} maxLength={2000}/></div><button className="btn primary" disabled={busy||!/^\+[1-9]\d{7,14}$/.test(to)||!text.trim()} onClick={()=>void sendTest()}><Send size={16}/> Enviar mensagem de teste</button></div>}
 {notice&&<div className="alert success">{notice}</div>}{error&&<p className="field-error">{error}</p>}
 <div className="modal-actions"><button className="btn secondary" onClick={()=>void refresh()}><RefreshCw size={16}/> Atualizar</button>{state?.status==='connected'?<button className="btn secondary" disabled={busy} onClick={()=>void disconnect()}><Unplug size={16}/> Desconectar</button>:<button className="btn primary" disabled={busy} onClick={()=>void connect()}>Iniciar conexão</button>}</div></>;
}
