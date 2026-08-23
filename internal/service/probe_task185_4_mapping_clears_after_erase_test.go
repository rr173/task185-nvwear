package service

import (
	"context"
	"testing"
	"task185-nvwear/internal/geometry"
	"task185-nvwear/internal/mapping"
	"task185-nvwear/internal/model"
	"task185-nvwear/internal/simulator"
	"task185-nvwear/internal/store"
)

func TestBug04_EraseAndReinitializeClearCurrentMappings(t *testing.T) {
	l,err:=geometry.NewLayout(1,1,4,4,512,1);if err!=nil{t.Fatal(err)};cfg:=&model.StorageConfig{ID:"cfg",MaxEraseCycles:10}
	blks:=make([]model.PhysicalBlock,0,l.TotalBlocks());for i:=0;i<l.TotalBlocks();i++{blks=append(blks,model.PhysicalBlock{BlockIndex:i,Status:model.BlockAvailable})}
	tbl:=mapping.NewTable();eng:=simulator.NewEngine(cfg,l,tbl,blks)
	if _,vio,err:=eng.Apply(context.Background(),1,model.PlanOperation{Type:model.OpWrite,LPN:1,DestPPN:0});err!=nil||vio!=nil{t.Fatalf("initial write: %v %v",err,vio)}
	if _,vio,err:=eng.Apply(context.Background(),2,model.PlanOperation{Type:model.OpErase,BlockIndex:0});err!=nil||vio!=nil{t.Fatalf("erase: %v %v",err,vio)}
	if _,vio,err:=eng.Apply(context.Background(),3,model.PlanOperation{Type:model.OpWrite,LPN:1,DestPPN:1});err!=nil||vio!=nil{t.Fatalf("rewrite after erase: %v %v",err,vio)}
	db,err:=store.Open(t.TempDir()+"/mapping.db");if err!=nil{t.Fatal(err)};defer db.Close();ms:=store.NewMappingStore(db)
	if err=ms.Insert(&model.LogicalMapping{ConfigID:"cfg",LPN:2,PPN:0,BlockIndex:0,Version:0});err!=nil{t.Fatal(err)};if err=ms.Reset("cfg");err!=nil{t.Fatal(err)};rows,err:=ms.LatestAll("cfg");if err!=nil{t.Fatal(err)};if len(rows)!=0{t.Fatalf("mapping reset left %d rows",len(rows))}
}
