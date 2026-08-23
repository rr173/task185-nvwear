package simulator

import (
	"context"
	"testing"
	"task185-nvwear/internal/geometry"
	"task185-nvwear/internal/mapping"
	"task185-nvwear/internal/model"
)

func TestBug10_ReservedBlocksRejectWriteAndRelocateTargets(t *testing.T) {
	l,err:=geometry.NewLayout(1,1,4,4,512,1);if err!=nil{t.Fatal(err)};reservedSet,err:=geometry.ReserveSet(l,1);if err!=nil{t.Fatal(err)};reserved:=reservedSet[0];blks:=make([]model.PhysicalBlock,0,l.TotalBlocks());for i:=0;i<l.TotalBlocks();i++{b:=model.PhysicalBlock{BlockIndex:i,Status:model.BlockAvailable};if i==reserved{b.IsReserved=true;b.Status=model.BlockReserved};blks=append(blks,b)}
	cfg:=&model.StorageConfig{ID:"cfg",MaxEraseCycles:10};tbl:=mapping.NewTable();eng:=NewEngine(cfg,l,tbl,blks)
	if _,vio,err:=eng.Apply(context.Background(),1,model.PlanOperation{Type:model.OpWrite,LPN:1,DestPPN:reserved*l.PagesPerBlock});err!=nil||vio==nil{t.Fatalf("reserved write: err=%v vio=%v",err,vio)}
	if _,vio,err:=eng.Apply(context.Background(),2,model.PlanOperation{Type:model.OpWrite,LPN:1,DestPPN:0});err!=nil||vio!=nil{t.Fatalf("source write: err=%v vio=%v",err,vio)};if tbl.Get(1)==nil{t.Fatal("source mapping missing")}
	if _,vio,err:=eng.Apply(context.Background(),3,model.PlanOperation{Type:model.OpRelocate,LPN:1,SrcPPN:0,DestPPN:reserved*l.PagesPerBlock});err!=nil||vio==nil{t.Fatalf("reserved relocate: err=%v vio=%v",err,vio)}
}
