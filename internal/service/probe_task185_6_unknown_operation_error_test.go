package service

import (
	"context"
	"errors"
	"testing"
	"task185-nvwear/internal/model"
	"task185-nvwear/internal/store"
)

func TestBug06_UnknownOperationReturnsDomainError(t *testing.T) {
	db,err:=store.Open(t.TempDir()+"/unknown.db");if err!=nil{t.Fatal(err)};defer db.Close();ctx:=context.Background();app:=New(db)
	cfg,err:=app.Configs.Create(ctx,ConfigCreateInput{Name:"cfg",DieCount:1,PlanesPerDie:1,BlocksPerPlane:4,PagesPerBlock:4,PageBytes:512,ReservedBlocks:1,MaxEraseCycles:10,WarnThreshold:80});if err!=nil{t.Fatal(err)};if _,err=app.Configs.Freeze(ctx,cfg.ID);err!=nil{t.Fatal(err)}
	p,err:=app.Plans.Create(ctx,PlanCreateInput{ConfigID:cfg.ID,Name:"bad-op",Ops:[]model.PlanOperation{{Type:"not-a-real-op"}}});if err!=nil{t.Fatal(err)};if _,err=app.Plans.Simulate(ctx,p.ID);err==nil||!errors.Is(err,model.ErrInvalidInput){t.Fatalf("want invalid-input error, got %v",err)}
}
