package geometry

import (
	"testing"
	"task185-nvwear/internal/model"
)

func TestBug08_EquivalentBlockOrdersProduceStableSnapshotHash(t *testing.T) {
	a:=[]model.PhysicalBlock{{BlockIndex:2,Status:model.BlockAvailable},{BlockIndex:0,Status:model.BlockBad},{BlockIndex:1,Status:model.BlockReserved,IsReserved:true}};b:=[]model.PhysicalBlock{{BlockIndex:1,Status:model.BlockReserved,IsReserved:true},{BlockIndex:2,Status:model.BlockAvailable},{BlockIndex:0,Status:model.BlockBad}};if BuildSnapshot(a).Hash!=BuildSnapshot(b).Hash{t.Fatal("same block state hashed differently")}
}
