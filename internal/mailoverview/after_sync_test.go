package mailoverview

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xynova/should-i-read/internal/pimalaya"
)

type recordingPage struct {
	html   []byte
	hashes []string
}

func (p *recordingPage) ReplacePage(html []byte, hashes []string) {
	p.html = append([]byte(nil), html...)
	p.hashes = append([]string(nil), hashes...)
}

func TestAfterSyncRebuildsPage(t *testing.T) {
	t.Parallel()
	page := &recordingPage{}
	after, err := CreateAfterSync(AfterSyncConfig{
		Classify: func(context.Context, *pimalaya.Result) (int, bool, error) {
			return 3, false, nil
		},
		Rebuild: func(context.Context) ([]byte, []string, error) {
			return []byte("<html>ok</html>"), []string{"h1"}, nil
		},
		Page: page,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res := &pimalaya.Result{Raw: []byte(`{"account":"gmail"}`)}
	line, err := after.Run(ctx, res)
	if err != nil {
		t.Fatal(err)
	}
	if line == "" {
		t.Fatal("empty line")
	}
	if string(page.html) != "<html>ok</html>" || len(page.hashes) != 1 {
		t.Fatalf("page html=%q hashes=%v", page.html, page.hashes)
	}
}

func TestAfterSyncClassifyError(t *testing.T) {
	t.Parallel()
	after, err := CreateAfterSync(AfterSyncConfig{
		Classify: func(context.Context, *pimalaya.Result) (int, bool, error) {
			return 0, false, errors.New("classify down")
		},
		Rebuild: func(context.Context) ([]byte, []string, error) {
			t.Fatal("rebuild should not run")
			return nil, nil, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = after.Run(ctx, &pimalaya.Result{Raw: []byte(`{"account":"gmail"}`)})
	if err == nil {
		t.Fatal("expected classify error")
	}
}
