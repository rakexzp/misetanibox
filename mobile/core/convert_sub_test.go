package mobilecore
import (
  "os"
  "testing"
)
func TestConvertSub1(t *testing.T) {
  b, err := os.ReadFile(`C:\Users\Egor\Desktop\Sub1-from-txt.txt`)
  if err != nil { t.Fatal(err) }
  out := ConvertSubscription(string(b), "[S1] ")
  n := ConvertSubscriptionCount(string(b), "[S1] ")
  t.Logf("count=%d outLen=%d", n, len(out))
  if n < 100 { t.Fatalf("too few proxies: %d", n) }
  if out == "" { t.Fatal("empty out") }
  if !contains(out, "hysteria2") { t.Fatal("no hysteria2 in out") }
  if !contains(out, "[S1] ") { t.Fatal("prefix missing") }
  t.Log(out[:min(300,len(out))])
}
func contains(s, sub string) bool { return len(s)>=len(sub) && (s==sub || len(sub)==0 || indexOf(s,sub)>=0) }
func indexOf(s, sub string) int {
  for i:=0;i+len(sub)<=len(s);i++ { if s[i:i+len(sub)]==sub { return i } }
  return -1
}
func min(a,b int) int { if a<b {return a}; return b }
