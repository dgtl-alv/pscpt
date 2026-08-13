import React, {useEffect, useState} from 'react';
import {createRoot} from 'react-dom/client';
import {BarChart3, Bike, Database, LockKeyhole, UploadCloud, Users} from 'lucide-react';
import './styles.css';

const api = async (path, body) => {
  const res = await fetch(path, {method: body ? 'POST' : 'GET', headers: {'Content-Type': 'application/json'}, body: body ? JSON.stringify(body) : undefined});
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || 'request failed');
  return data;
};

function App(){
  const [user,setUser]=useState(null); const [view,setView]=useState('landing'); const [msg,setMsg]=useState('');
  useEffect(()=>{api('/api/auth/me').then(d=>{setUser(d.user); setView('dashboard')}).catch(()=>{})},[]);
  const authed = (u)=>{setUser(u); setView('dashboard'); setMsg('')};
  return <>
    <header className="topbar"><div className="brand"><span>PSCPT</span><small>PS Correlation Platform</small></div><nav>{user?<><button onClick={()=>setView('dashboard')}>Dashboard</button><button onClick={()=>setView('uploads')}>Uploads</button><button onClick={()=>setView('change')}>Change Password</button><button onClick={async()=>{await api('/api/auth/logout',{});setUser(null);setView('landing')}}>Logout</button></>:<><button onClick={()=>setView('landing')}>Home</button><button onClick={()=>setView('login')}>Login</button><button className="solid" onClick={()=>setView('register')}>Register</button></>}</nav></header>
    {msg && <div className="toast">{msg}</div>}
    {view==='landing' && <Landing go={setView}/>} {view==='login' && <Auth mode="login" done={authed}/>} {view==='register' && <Auth mode="register" done={authed}/>} {view==='forgot' && <Forgot setMsg={setMsg}/>} {view==='change' && <Change setMsg={setMsg}/>} {view==='dashboard' && <Dashboard user={user}/>} {view==='uploads' && <Uploads/>} 
    {(view==='login'||view==='register') && <p className="switch">Forgot password? <button onClick={()=>setView('forgot')}>Reset here</button></p>}
  </>;
}
function Landing({go}){return <main className="hero"><section><p className="eyebrow">offline-ready internal analytics</p><h1>Understand PS performance from people, learning, leads, and sales.</h1><p className="lead">PSCPT builds PS-month correlation reports from Talenta, Moodle, Emica lead, Emica sales order, and manual leadership/factor uploads.</p><div className="actions"><button className="solid big" onClick={()=>go('register')}>Start Workspace</button><button className="ghost big" onClick={()=>go('login')}>Login</button></div></section><section className="grid">{[[Users,'Individual Factor','Manual Excel for profile and background'],[Database,'Capability','Talenta, Moodle, and lead behavior'],[Bike,'Sales Performance','Emica sale.order by bike type'],[BarChart3,'Correlation Engine','Pearson, Spearman, driver analysis'],[UploadCloud,'Upload Center','Validate columns, periods, and PS match'],[LockKeyhole,'Access Control','Register, login, forgot, change password']].map(([Icon,t,d])=><article className="card" key={t}><Icon/><h3>{t}</h3><p>{d}</p></article>)}</section></main>}
function Auth({mode,done}){const [f,setF]=useState({name:'',email:'',password:''}); const [err,setErr]=useState(''); const submit=async e=>{e.preventDefault();setErr('');try{const d=await api(`/api/auth/${mode}`,f);done(d.user)}catch(x){setErr(x.message)}};return <Form title={mode==='login'?'Welcome back':'Create PSCPT account'} onSubmit={submit} err={err}>{mode==='register'&&<input placeholder="Name" value={f.name} onChange={e=>setF({...f,name:e.target.value})}/>}<input placeholder="Email" type="email" value={f.email} onChange={e=>setF({...f,email:e.target.value})}/><input placeholder="Password min 8 chars" type="password" value={f.password} onChange={e=>setF({...f,password:e.target.value})}/><button className="solid">{mode==='login'?'Login':'Register'}</button></Form>}
function Forgot({setMsg}){const [email,setEmail]=useState(''); const [link,setLink]=useState('');return <Form title="Forgot password" onSubmit={async e=>{e.preventDefault();const d=await api('/api/auth/forgot-password',{email});setLink(d.reset_link||'');setMsg('Reset link generated for local dev')}}><input placeholder="Email" value={email} onChange={e=>setEmail(e.target.value)}/><button className="solid">Generate Reset Link</button>{link&&<code className="resetlink">{link}</code>}</Form>}
function Change({setMsg}){const [f,setF]=useState({currentPassword:'',newPassword:''});return <Form title="Change password" onSubmit={async e=>{e.preventDefault();await api('/api/auth/change-password',f);setMsg('Password changed')}}><input type="password" placeholder="Current password" onChange={e=>setF({...f,currentPassword:e.target.value})}/><input type="password" placeholder="New password min 8 chars" onChange={e=>setF({...f,newPassword:e.target.value})}/><button className="solid">Change Password</button></Form>}
function Form({title,onSubmit,children,err}){return <main className="auth"><form onSubmit={onSubmit}><h2>{title}</h2>{children}{err&&<p className="error">{err}</p>}</form></main>}

function Uploads(){
  const [type,setType]=useState('individual_factor'); const [file,setFile]=useState(null); const [result,setResult]=useState(null); const [items,setItems]=useState([]); const [err,setErr]=useState('');
  const load=()=>api('/api/uploads/manual/list').then(d=>setItems(d.uploads||[])).catch(()=>{});
  useEffect(load,[]);
  const submit=async e=>{e.preventDefault();setErr('');setResult(null);if(!file){setErr('Choose .xlsx file');return} const fd=new FormData();fd.append('type',type);fd.append('file',file);try{const res=await fetch('/api/uploads/manual',{method:'POST',body:fd});const data=await res.json();if(!res.ok)throw new Error(data.error||'upload failed');setResult(data);load()}catch(x){setErr(x.message)}};
  return <main className="dash"><h1>Manual Uploads</h1><p>Upload individual factor dan leadership Excel. Header baris pertama jadi nama kolom otomatis.</p><form className="uploadbox" onSubmit={submit}><select value={type} onChange={e=>setType(e.target.value)}><option value="individual_factor">Individual Factor</option><option value="leadership">Leadership</option></select><input type="file" accept=".xlsx" onChange={e=>setFile(e.target.files?.[0])}/><button className="solid">Upload & Preview</button>{err&&<p className="error">{err}</p>}</form>{result&&<section className="panel"><h2>Uploaded: {result.filename}</h2><p>{result.row_count} rows. Columns: {result.columns.join(', ')}</p><pre>{JSON.stringify(result.preview,null,2)}</pre></section>}<section className="panel"><h2>Recent Uploads</h2>{items.length===0?<p>No uploads yet.</p>:<table><thead><tr><th>Type</th><th>File</th><th>Rows</th><th>Created</th></tr></thead><tbody>{items.map(x=><tr key={x.id}><td>{x.type}</td><td>{x.filename}</td><td>{x.row_count}</td><td>{new Date(x.created_at).toLocaleString()}</td></tr>)}</tbody></table>}</section></main>
}

function Dashboard({user}){const [summary,setSummary]=useState(null);useEffect(()=>{api('/api/dashboard/summary').then(setSummary)},[]);return <main className="dash"><h1>Dashboard Overview</h1><p>Halo {user?.name}. Engine siap untuk PS-month pipeline.</p><div className="metricrow">{(summary?.cards||[]).map(c=><article className={'metric '+c.tone} key={c.label}><b>{c.value}</b><span>{c.label}</span></article>)}</div><section className="panel"><h2>Pipeline V1</h2><ol><li>Upload individual factor and leadership Excel.</li><li>Sync Talenta, Moodle, Emica lead, Emica sales order.</li><li>Build PS-month master table.</li><li>Run correlation, driver analysis, segmentation, early warning.</li></ol></section></main>}
createRoot(document.getElementById('root')).render(<App/>);
