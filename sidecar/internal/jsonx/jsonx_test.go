package jsonx

import (
	"encoding/json"
	"errors"
	"testing"
)

const settingsFixture = `{
  "attribution": {
    "commit": "",
    "pr": ""
  },
  "model": "opus",
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/git-status.sh",
            "timeout": 10
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/notify-done.sh",
            "timeout": 10
          }
        ]
      }
    ],
    "Notification": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "~/.claude/hooks/notify-done.sh",
            "timeout": 10
          }
        ]
      }
    ]
  },
  "enabledPlugins": {
    "superpowers@claude-plugins-official": true,
    "andrej-karpathy-skills@karpathy-skills": false
  },
  "extraKnownMarketplaces": {
    "karpathy-skills": {
      "source": {
        "source": "github",
        "repo": "forrestchang/andrej-karpathy-skills"
      }
    }
  },
  "tui": "fullscreen",
  "theme": "auto",
  "inputNeededNotifEnabled": true
}
`

func roundTrip(t *testing.T, in string) string {
	t.Helper()
	o, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return string(Marshal(o))
}

func TestRoundTripKeepsOrder(t *testing.T) {
	if got := roundTrip(t, settingsFixture); got != settingsFixture {
		t.Fatalf("round trip differs:\n%s", got)
	}
}

func TestNoHTMLEscapingAndUnicode(t *testing.T) {
	want := "{\n  \"c\": \"a && b > /tmp/x < y 🌿 é\"\n}\n"
	if got := roundTrip(t, `{"c":"a && b > /tmp/x < y 🌿 é"}`); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestNumbersKept(t *testing.T) {
	want := "{\n  \"t\": 10,\n  \"f\": 1.50,\n  \"big\": 12345678901234567890\n}\n"
	if got := roundTrip(t, `{"t": 10, "f": 1.50, "big": 12345678901234567890}`); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSyntaxErrorPosition(t *testing.T) {
	_, err := Parse([]byte("{\n  \"a\": 1,\n}"))
	var se *SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("want *SyntaxError, got %T %v", err, err)
	}
	if se.Line != 3 || se.Col != 1 {
		t.Fatalf("Line,Col = %d,%d, want 3,1 (%+v)", se.Line, se.Col, se)
	}
}

func TestRootMustBeObject(t *testing.T) {
	if _, err := Parse([]byte("[]")); err == nil {
		t.Fatal("expected error")
	}
}

func TestEmptyContainersAndNull(t *testing.T) {
	in := "{\n  \"a\": {},\n  \"b\": [],\n  \"c\": null,\n  \"d\": [\n    1,\n    []\n  ]\n}\n"
	if got := roundTrip(t, in); got != in {
		t.Fatalf("got %q", got)
	}
}

func TestDuplicateKeyLastWinsFirstPosition(t *testing.T) {
	want := "{\n  \"a\": 3,\n  \"b\": 2\n}\n"
	if got := roundTrip(t, `{"a":1,"b":2,"a":3}`); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestSetGetDelete(t *testing.T) {
	o := NewObject()
	o.Set("x", "1")
	o.Set("y", true)
	o.Set("x", "2")
	if v, ok := o.Get("x"); !ok || v != "2" {
		t.Fatalf("Get x = %v %v", v, ok)
	}
	o.Delete("x")
	o.Delete("missing")
	if _, ok := o.Get("x"); ok || len(o.Keys) != 1 || o.Keys[0] != "y" {
		t.Fatalf("after delete: %+v", o)
	}
}

func TestTrailingGarbageRejected(t *testing.T) {
	if _, err := Parse([]byte(`{} x`)); err == nil {
		t.Fatal("expected error")
	}
}

func wantSyntax(t *testing.T, in string, line, col int) {
	t.Helper()
	_, err := Parse([]byte(in))
	var se *SyntaxError
	if !errors.As(err, &se) {
		t.Fatalf("want *SyntaxError, got %T %v", err, err)
	}
	if se.Line != line || se.Col != col {
		t.Fatalf("Line,Col = %d,%d, want %d,%d (%v)", se.Line, se.Col, line, col, se)
	}
}

func TestRejectsInvalidUTF8(t *testing.T) {
	wantSyntax(t, "{\n  \"a\": \"x\xffy\"\n}", 2, 10)
}

func TestRejectsLoneSurrogate(t *testing.T) {
	wantSyntax(t, `{`+"\n"+`  "a": "\ud800"}`, 2, 9)
	wantSyntax(t, `{"a": "\udc00x"}`, 1, 8)
	wantSyntax(t, `{"a": "\ud800\u0041"}`, 1, 8)
}

func TestValidSurrogatePairKept(t *testing.T) {
	want := "{\n  \"a\": \"🌿\"\n}\n"
	if got := roundTrip(t, `{"a":"\ud83c\udf3f"}`); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestZeroObjectSet(t *testing.T) {
	var o Object
	o.Set("a", json.Number("1"))
	if got := string(Marshal(&o)); got != "{\n  \"a\": 1\n}\n" {
		t.Fatalf("got %q", got)
	}
}
