package appcore

import "testing"

func TestLiteServersKeepGroupsAndProviders(t *testing.T) {
	data := []byte(`proxies:
  - {name: Node, type: ss, server: secret.example, password: secret}
proxy-groups:
  - name: Main
    type: select
    proxies: [Auto, Europe, Node]
    use: [provider]
  - {name: Auto, type: url-test, proxies: [Node]}
  - {name: Europe, type: select, proxies: [Node]}
rules: ["MATCH,Main"]
`)
	got, err := parseLiteServers("profile-b", data, map[string]string{"Main": "Europe"})
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileID != "profile-b" || got.Selector != "Main" {
		t.Fatalf("wrong profile/selector: %+v", got)
	}
	if len(got.Groups) != 3 || got.Groups[0].Selected != "Europe" {
		t.Fatalf("groups lost: %+v", got.Groups)
	}
	if len(got.Groups[0].Members) != 3 || len(got.Groups[0].Providers) != 1 {
		t.Fatal("group/provider references lost")
	}
	if len(got.Nodes) != 1 || got.Nodes[0].Name != "Node" {
		t.Fatalf("nodes: %+v", got.Nodes)
	}
}

func TestLiteServersRejectStaleSelection(t *testing.T) {
	got, err := parseLiteServers("b", []byte("proxy-groups:\n  - {name: Main, type: select, proxies: [New]}\nrules: [\"MATCH,Main\"]\n"), map[string]string{"Main": "Old"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Groups[0].Selected != "New" {
		t.Fatalf("stale node retained: %+v", got)
	}
}

func TestLiteMainSelectorLastMatchAndOptions(t *testing.T) {
	got, err := parseLiteServers("b", []byte("rules: [\"MATCH,Old\", \"MATCH, New, no-resolve\"]\n"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Selector != "New" {
		t.Fatalf("selector = %q", got.Selector)
	}
}

func TestLiteServersRejectMalformedYAML(t *testing.T) {
	if _, err := parseLiteServers("b", []byte("["), nil); err == nil {
		t.Fatal("expected error")
	}
}
