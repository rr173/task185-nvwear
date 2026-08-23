package service

import (
	"context"
	"testing"
	"task185-nvwear/internal/model"
	"task185-nvwear/internal/store"
)

func TestBug03_PlanIdentityKeepsOperationTypeAndConfigScope(t *testing.T) {
	db, err := store.Open(t.TempDir()+"/hash.db"); if err != nil { t.Fatal(err) }; defer db.Close()
	ctx := context.Background(); app := New(db)
	makeCfg := func(name string) *model.StorageConfig { c,e:=app.Configs.Create(ctx,ConfigCreateInput{Name:name,DieCount:1,PlanesPerDie:1,BlocksPerPlane:8,PagesPerBlock:4,PageBytes:512,ReservedBlocks:1,MaxEraseCycles:10,WarnThreshold:80}); if e!=nil {t.Fatal(e)}; if _,e=app.Configs.Freeze(ctx,c.ID);e!=nil{t.Fatal(e)}; return c }
	cfg1,cfg2:=makeCfg("one"),makeCfg("two")
	write:=[]model.PlanOperation{{Type:model.OpWrite,LPN:1,DestPPN:0}}; erase:=[]model.PlanOperation{{Type:model.OpErase,BlockIndex:0}}
	p1,err:=app.Plans.Create(ctx,PlanCreateInput{ConfigID:cfg1.ID,Name:"write",Ops:write});if err!=nil{t.Fatal(err)}
	p2,err:=app.Plans.Create(ctx,PlanCreateInput{ConfigID:cfg1.ID,Name:"erase",Ops:erase});if err!=nil{t.Fatal(err)}
	if p1.ID==p2.ID{t.Fatalf("different operation types were merged: %s",p1.ID)}
	p3,err:=app.Plans.Create(ctx,PlanCreateInput{ConfigID:cfg1.ID,Name:"write-dup",Ops:write});if err!=nil{t.Fatal(err)};if p3.ID!=p1.ID{t.Fatal("same sequence was not deduplicated")}
	p4,err:=app.Plans.Create(ctx,PlanCreateInput{ConfigID:cfg2.ID,Name:"write-other",Ops:write});if err!=nil{t.Fatal(err)};if p4.ID==p1.ID{t.Fatal("same hash crossed config boundary")}
}
