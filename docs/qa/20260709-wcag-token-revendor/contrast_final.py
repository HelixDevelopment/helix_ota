#!/usr/bin/env python3
def lin(c):
    c=c/255.0; return c/12.92 if c<=0.03928 else ((c+0.055)/1.055)**2.4
def L(h):
    h=h.lstrip('#'); return 0.2126*lin(int(h[0:2],16))+0.7152*lin(int(h[2:4],16))+0.0722*lin(int(h[4:6],16))
def ratio(f,b):
    a,c=L(f),L(b);
    if a<c: a,c=c,a
    return (a+0.05)/(c+0.05)
def rgb(h): h=h.lstrip('#'); return (int(h[0:2],16),int(h[2:4],16),int(h[4:6],16))
def hx(t): return '#%02x%02x%02x'%t
def over(col,par,a):
    c,p=rgb(col),rgb(par); return hx(tuple(round(a*c[i]+(1-a)*p[i]) for i in range(3)))
def row(fg,surfs,thr):
    parts=[]
    for nm,bg in surfs:
        r=ratio(fg,bg); parts.append(f"{nm}={r:.2f}{'✓' if r>=thr else '✗'}")
    print(f"  {fg}  thr{thr}: "+"  ".join(parts))
print("FINAL CHOSEN VALUES — measured ratios on REAL rendered surfaces")
print("\nLIGHT --muted #64748b -> #475569 (text 4.5; renders on bg + warm panels)")
row('#475569',[('bg','#ffffff'),('warm','#f1f5f9')],4.5)
print("\nLIGHT --warn #eab308 -> #854d0e (text 4.5; bg + card + badge/alert tint)")
row('#854d0e',[('bg','#ffffff'),('warm','#f1f5f9'),('badgeTint',over('#854d0e','#ffffff',0.15)),('alertTint',over('#854d0e','#ffffff',0.08))],4.5)
print("\nLIGHT --success #16a34a -> #166534 (text 4.5; bg + card + badge tint)")
row('#166534',[('bg','#ffffff'),('warm','#f1f5f9'),('badgeTint',over('#166534','#ffffff',0.15))],4.5)
print("\nDARK --danger #7f1d1d -> #ef4444 (text 4.5; surface #020817 + badge/alert tint)")
row('#ef4444',[('surface','#020817'),('badgeTint',over('#ef4444','#020817',0.15)),('alertTint',over('#ef4444','#020817',0.08))],4.5)
print("\nNEW --border-strong #64748b (UI boundary 3.0; both themes, all surfaces)")
row('#64748b',[('L/bg','#ffffff'),('L/warm','#f1f5f9'),('D/bg','#020817'),('D/warm','#1e293b')],3.0)
print("\nDARK-PRESERVATION (explicit dark overrides so light fix doesn't leak) — must still PASS")
print("dark --warn stays #eab308 (bright amber on dark)")
row('#eab308',[('surface','#020817'),('badgeTint',over('#eab308','#020817',0.15))],4.5)
print("dark --success stays #16a34a")
row('#16a34a',[('surface','#020817'),('badgeTint',over('#16a34a','#020817',0.15))],4.5)
print("\nUNCHANGED (out of scope) — confirm still pass on plain rendered surface")
print("light --danger #dc2626 (text on bg + badge tint)")
row('#dc2626',[('bg','#ffffff'),('badgeTint',over('#dc2626','#ffffff',0.15))],4.5)
print("dark --muted #94a3b8"); row('#94a3b8',[('surface','#020817')],4.5)
