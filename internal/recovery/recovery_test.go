package recovery

import (
	"testing"

	"task185-nvwear/internal/model"
)

func TestValidatePowerLossNoCheckpoint(t *testing.T) {
	v := New(nil)
	ops := []model.PlanOperation{
		{Seq: 1, Type: model.OpWrite},
		{Seq: 2, Type: model.OpPowerLoss},
	}
	vio := v.Validate(ops, map[int]bool{})
	if vio == nil || vio.Type != model.VioPowerLossNoCheck {
		t.Fatalf("应产生无检查点掉电违规: %+v", vio)
	}
}

func TestValidatePowerLossWithCheckpoint(t *testing.T) {
	v := New(nil)
	ops := []model.PlanOperation{
		{Seq: 1, Type: model.OpWrite},
		{Seq: 2, Type: model.OpCheckpoint},
		{Seq: 3, Type: model.OpWrite},
		{Seq: 4, Type: model.OpPowerLoss},
	}
	if vio := v.Validate(ops, map[int]bool{2: true}); vio != nil {
		t.Fatalf("有检查点不应违规: %+v", vio)
	}
}

func TestValidateDanglingCheckpoint(t *testing.T) {
	v := New(nil)
	ops := []model.PlanOperation{
		{Seq: 1, Type: model.OpWrite},
	}
	vio := v.Validate(ops, map[int]bool{99: true})
	if vio == nil || vio.Type != model.VioCheckpointDangling {
		t.Fatalf("应产生检查点悬空违规: %+v", vio)
	}
}

func TestRecoverable(t *testing.T) {
	ops := []model.PlanOperation{
		{Seq: 1, Type: model.OpWrite},
		{Seq: 2, Type: model.OpCheckpoint},
	}
	ok, cp := Recoverable(ops)
	if !ok || cp != 2 {
		t.Fatalf("Recoverable = %v/%d", ok, cp)
	}
	ok, _ = Recoverable([]model.PlanOperation{{Seq: 1, Type: model.OpWrite}})
	if ok {
		t.Fatal("无检查点应不可恢复")
	}
}

func TestReplay(t *testing.T) {
	ops := []model.PlanOperation{
		{Seq: 1, Type: model.OpWrite},
		{Seq: 2, Type: model.OpCheckpoint},
		{Seq: 3, Type: model.OpWrite},
		{Seq: 4, Type: model.OpErase},
		{Seq: 5, Type: model.OpPowerLoss},
	}
	v := New(nil)
	replayed, ok, err := v.Replay(ops, 2)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if !ok || replayed != 2 {
		t.Fatalf("Replay = %d/%v", replayed, ok)
	}
}

func TestReplayMissingCursor(t *testing.T) {
	ops := []model.PlanOperation{{Seq: 1, Type: model.OpWrite}}
	v := New(nil)
	if _, _, err := v.Replay(ops, 2); err == nil {
		t.Fatal("游标不存在应报错")
	}
}
