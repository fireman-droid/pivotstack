package proxy

import (
	"hash/fnv"
	"math/rand"
)

// daoguiNamePool — 道诡异仙 themed aliases used as synthetic leaderboard entries.
// Mix of canonical names and atmospheric handles.
var daoguiNamePool = []string{
	"烛龙归墟", "墨君衍", "白林秋荻", "鬼伶仃", "李火旺",
	"半身", "怀沙", "萧仁", "沈炼", "白霖君",
	"夜行尸", "朱砂客", "墨衣道君", "断头郎", "无相婆",
	"红衣使者", "守墓人", "蛛网仙", "枯骨真人", "镜中人",
	"听雨翁", "三魂客", "九幽散人", "长生眷", "纸人公子",
	"血灯婆", "孤坟客", "哑女", "墙里人", "倒悬子",
	"画皮娘", "招魂郎", "幽兰使", "蜃楼客", "焚棺者",
}

// fnvSeed returns a deterministic int64 seed from a string.
func fnvSeed(s string) int64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return int64(h.Sum64())
}

// generateFakes produces N synthetic LeaderEntry rows anchored above realTop1.
// Stable for the same dateSeed (so users see consistent fakes within a UTC day).
//
// Anchor: firstFake = realTop1 * (1.05 .. 1.50), each next *= 0.78 .. 0.93
func generateFakes(metric string, realTop1 float64, n int, dateSeed string) []LeaderEntry {
	if n <= 0 || realTop1 <= 0 {
		return nil
	}
	if n > len(daoguiNamePool) {
		n = len(daoguiNamePool)
	}
	rng := rand.New(rand.NewSource(fnvSeed(dateSeed)))

	// shuffle name pool (Fisher-Yates) and take first N
	pool := make([]string, len(daoguiNamePool))
	copy(pool, daoguiNamePool)
	for i := len(pool) - 1; i > 0; i-- {
		j := rng.Intn(i + 1)
		pool[i], pool[j] = pool[j], pool[i]
	}

	out := make([]LeaderEntry, 0, n)
	v := realTop1 * (1.05 + rng.Float64()*0.45)
	for i := 0; i < n; i++ {
		// quantize: tokens/requests should look integer-ish
		val := v
		switch metric {
		case "requests", "tokens":
			val = float64(int64(val))
		case "credits":
			// credits look like floats with 4 decimal places
			val = float64(int64(val*10000)) / 10000.0
		}
		out = append(out, LeaderEntry{
			Alias:  pool[i],
			Value:  val,
			IsFake: true,
		})
		v *= 0.78 + rng.Float64()*0.15
		if v < 0 {
			v = 0
		}
	}
	return out
}
