package mapping

import "encoding/json"

// jsonMarshal 序列化辅助（避免包内重复 import）。
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
