#!/usr/bin/env bash
# Local end-to-end check against a freshly built image. Usage: scripts/smoke.sh [image]
set -u
IMG=${1:-shitmail:local}
docker rm -f shitmail-smoke >/dev/null 2>&1
docker run -d --name shitmail-smoke -p 18080:8080 -p 12525:2525 \
  -e MAILTUB_DOMAIN=shit.example -e DROP_ATTACHMENTS=true -e LOG_LEVEL=debug "$IMG" >/dev/null
sleep 2
H=(-H 'X-Authentik-Uid: 42' -H 'X-Authentik-Username: marc')
A=http://localhost:18080/api/v1
j() { python3 -c "import sys,json;d=json.load(sys.stdin);print($1)"; }
echo "no headers  -> $(curl -s -o /dev/null -w '%{http_code}' $A/mailboxes)  (want 401)"
echo "me          -> $(curl -s "${H[@]}" $A/me)"
ADDR=$(curl -s "${H[@]}" -X POST -H 'Content-Type: application/json' -d '{"localPart":"slack1","ttlHours":168}' $A/mailbox | j 'd["address"]')
echo "create      -> $ADDR"
echo "dup create  -> $(curl -s -o /dev/null -w '%{http_code}' "${H[@]}" -X POST -H 'Content-Type: application/json' -d '{"localPart":"slack1"}' $A/mailbox)  (want 409)"
echo "random      -> $(curl -s "${H[@]}" -X POST $A/mailbox | j 'd["address"]')"
echo "list uid42  -> $(curl -s "${H[@]}" $A/mailboxes | j '[m["address"] for m in d["mailboxes"]]')"
echo "list uid43  -> $(curl -s -H 'X-Authentik-Uid: 43' $A/mailboxes)"
echo "uid43 reads -> $(curl -s -o /dev/null -w '%{http_code}' -H 'X-Authentik-Uid: 43' "$A/mailbox/$ADDR")  (want 404)"
python3 - "$ADDR" <<'PY'
import smtplib,sys
from email.message import EmailMessage
for to in (sys.argv[1], "nobody@shit.example"):
    m=EmailMessage(); m["From"]="slack@example.com"; m["To"]=to; m["Subject"]="hello "+to
    m.set_content("plain body")
    m.add_alternative('<html><body><img src="https://tracker.example.com/p.gif"><p>html body</p></body></html>', subtype='html')
    m.add_attachment(b"PDFDATA"*100, maintype="application", subtype="pdf", filename="x.pdf")
    s=smtplib.SMTP("localhost",12525); r=s.send_message(m); s.quit(); print("smtp        ->",to,"accepted, refused:",r)
PY
sleep 1
curl -s "${H[@]}" "$A/mailbox/$ADDR/emails" | j '"emails      -> total %d; "%d["total"] + "; ".join("%s | attachments=%d"%(e["subject"],len(e.get("attachments") or [])) for e in d["emails"])'
for u in 'http://127.0.0.1:8080/x' 'http://100.64.0.1/x' 'ftp://a/b' 'https://example.com/'; do
  printf 'proxy %-26s -> %s\n' "$u" "$(curl -s -o /dev/null -w '%{http_code}' "${H[@]}" "$A/proxy?u=$(python3 -c 'import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1],safe=""))' "$u")")"
done
echo "admin       -> non-admin $(curl -s -o /dev/null -w '%{http_code}' "${H[@]}" localhost:18080/admin/api/stats), admin $(curl -s -o /dev/null -w '%{http_code}' "${H[@]}" -H 'X-Authentik-Groups: authentik Admins' localhost:18080/admin/api/stats)  (want 403, 401/200)"
echo "health cmd  -> $(docker exec shitmail-smoke /app/shitmail health && echo ok)"
echo "delete      -> $(curl -s -o /dev/null -w '%{http_code}' "${H[@]}" -X DELETE "$A/mailbox/$ADDR"), left: $(curl -s "${H[@]}" $A/mailboxes | j 'len(d["mailboxes"])')"
echo "log         -> $(docker logs shitmail-smoke 2>&1 | grep -c 'no active mailbox') unknown-address drops"
docker rm -f shitmail-smoke >/dev/null
