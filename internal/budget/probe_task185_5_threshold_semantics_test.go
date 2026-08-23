package budget

import (
	"context"
	"testing"
	"task185-nvwear/internal/geometry"
	"task185-nvwear/internal/mapping"
	"task185-nvwear/internal/model"
	"task185-nvwear/internal/simulator"
)

func TestBug05_ExactWearThresholdIsAViolationEverywhere(t *testing.T) {
	e,err:=New(10,80,0,1,80);if err!=nil{t.Fatal(err)};blocks:=[]model.PhysicalBlock{{BlockIndex:0,Status:model.BlockAvailable,EraseCount:8}};if vio:=e.Check(8,blocks);vio==nil{t.Fatal("exact hot-block threshold was accepted")};if got:=geometry.WearTier(80,80);got!="warning"{t.Fatalf("wear tier=%s",got)}
	l,err:=geometry.NewLayout(1,1,2,4,512,0);if err!=nil{t.Fatal(err)};cfg:=&model.StorageConfig{ID:"cfg",MaxEraseCycles:10};eng:=simulator.NewEngine(cfg,l,mapping.NewTable(),[]model.PhysicalBlock{{BlockIndex:0,Status:model.BlockAvailable}})
	for i:=0;i<10;i++{if _,vio,err:=eng.Apply(context.Background(),i+1,model.PlanOperation{Type:model.OpErase,BlockIndex:0});err!=nil||vio!=nil{t.Fatalf("erase %d: %v %v",i,err,vio)}};if eng.Blocks[0].Status!=model.BlockBad{t.Fatalf("threshold erase did not transition block: %s",eng.Blocks[0].Status)}
}
