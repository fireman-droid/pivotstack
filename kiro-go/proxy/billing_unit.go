package proxy

// QuotaToPivotDollars 把 new-api upstream quota 转成 PivotStack 虚拟$。
// 所有单位字段都由 reservation snapshot 传入，避免 admin 改单位/markup 影响在途请求。
func QuotaToPivotDollars(
	quota int64,
	quotaPerUnitDollar float64,
	yuanPerUpstreamDollar float64,
	pivotStackDollarsPerYuanSnap float64,
	markup float64,
) float64 {
	if quota <= 0 || quotaPerUnitDollar == 0 || yuanPerUpstreamDollar == 0 || pivotStackDollarsPerYuanSnap == 0 || markup == 0 {
		return 0
	}
	return float64(quota) / quotaPerUnitDollar * yuanPerUpstreamDollar * pivotStackDollarsPerYuanSnap * markup
}
