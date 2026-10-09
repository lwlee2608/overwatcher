package handler

import (
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCLIInstaller(t *testing.T) {
	for _, tag := range []string{"v1.2.3", "", "v1.2.3'; echo unexpected; #"} {
		t.Run(tag, func(t *testing.T) {
			r := gin.New()
			r.GET("/cli.sh", NewCLIHandler(tag).Serve)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/cli.sh", nil))
			if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "text/x-shellscript; charset=utf-8" {
				t.Fatalf("unexpected response: %d %v", w.Code, w.Header())
			}
			body := w.Body.String()
			if strings.Contains(body, "{{RELEASE_TAG}}") {
				t.Fatal("unresolved release tag")
			}
			want := tag
			if want == "" {
				want = "latest"
			}
			line := strings.Split(body, "\n")[4]
			cmd := exec.Command("sh", "-c", line+"; printf '%s' \"$RELEASE_TAG\"")
			out, err := cmd.CombinedOutput()
			if err != nil || string(out) != want {
				t.Fatalf("release tag: %q, %v; want %q", out, err, want)
			}
			cmd = exec.Command("sh", "-n")
			cmd.Stdin = strings.NewReader(body)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("invalid POSIX shell: %s, %v", out, err)
			}
		})
	}
}
