package main

func fractionOfCapacity(requested, capacity int64) float64 {
    if capacity == 0 {
        return 1
    }
    return float64(requested) / float64(capacity)
}

func getBalancedResourceScore(cFraction, mFraction, pFraction float64) float64 {
    mean := (cFraction + mFraction + pFraction) / float64(3)
    cRatio := (cFraction - mean) * (cFraction - mean)
    mRatio := (mFraction - mean) * (mFraction - mean)
    pRatio := (pFraction - mean) * (pFraction - mean)

    variance := float64((cRatio + mRatio + pRatio) / float64(3))
    return (1 - variance) * float64(MaxPriority)
}

func getLeastRequestedScore(requested, capacity int64) float64 {
    if capacity == 0 || requested > capacity {
        return 0
    }
    return float64(capacity - requested) * float64(MaxPriority) / float64(capacity)
}
