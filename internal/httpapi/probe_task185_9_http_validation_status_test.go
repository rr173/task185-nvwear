package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"task185-nvwear/internal/service"
	"task185-nvwear/internal/store"
)

func TestBug09_InvalidJSONUsesConflictStatusOnAllWriteEndpoints(t *testing.T) {
	db,err:=store.Open(t.TempDir()+"/http.db");if err!=nil{t.Fatal(err)};defer db.Close();srv:=NewServer(service.New(db))
	for _,path:=range []string{"/api/configs","/api/plans"}{r:=httptest.NewRequest(http.MethodPost,path,strings.NewReader(`{"unknown":`));rr:=httptest.NewRecorder();srv.Handler().ServeHTTP(rr,r);if rr.Code!=http.StatusConflict{t.Fatalf("%s status=%d body=%s",path,rr.Code,rr.Body.String())}}
}
