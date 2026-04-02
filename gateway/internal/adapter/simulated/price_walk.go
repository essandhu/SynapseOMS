package simulated

import (
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// PriceWalk generates synthetic prices using Geometric Brownian Motion (GBM).
// dS = S * (mu*dt + sigma*sqrt(dt)*Z) where Z ~ N(0,1)
type PriceWalk struct {
	mu           sync.RWMutex
	currentPrice decimal.Decimal
	volatility   float64 // annualized: 0.30 equity, 0.80 crypto
	drift        float64 // annualized drift (mu)
	interval     time.Duration
	ticker       *time.Ticker
	stopCh       chan struct{}
	rng          *rand.Rand
}

// NewPriceWalk creates a new PriceWalk with the given initial price, volatility, drift, and tick interval.
func NewPriceWalk(initialPrice decimal.Decimal, volatility, drift float64, interval time.Duration) *PriceWalk {
	return &PriceWalk{
		currentPrice: initialPrice,
		volatility:   volatility,
		drift:        drift,
		interval:     interval,
		stopCh:       make(chan struct{}),
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Start begins the price walk ticker.
func (pw *PriceWalk) Start() {
	pw.ticker = time.NewTicker(pw.interval)
	go func() {
		for {
			select {
			case <-pw.ticker.C:
				pw.step()
			case <-pw.stopCh:
				pw.ticker.Stop()
				return
			}
		}
	}()
}

// Stop halts the price walk ticker.
func (pw *PriceWalk) Stop() {
	select {
	case pw.stopCh <- struct{}{}:
	default:
	}
}

// CurrentPrice returns the latest synthetic price.
func (pw *PriceWalk) CurrentPrice() decimal.Decimal {
	pw.mu.RLock()
	defer pw.mu.RUnlock()
	return pw.currentPrice
}

// Step performs one GBM step. Exported for testing.
func (pw *PriceWalk) Step() {
	pw.step()
}

func (pw *PriceWalk) step() {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	// dt in years: interval / (365.25 * 24h)
	dt := pw.interval.Seconds() / (365.25 * 24 * 3600)
	z := pw.randNormal()
	dS := pw.currentPrice.InexactFloat64() * (pw.drift*dt + pw.volatility*math.Sqrt(dt)*z)
	newPrice := pw.currentPrice.Add(decimal.NewFromFloat(dS))
	if newPrice.IsPositive() {
		pw.currentPrice = newPrice
	}
	// If newPrice <= 0, keep current price (price floor at current value).
}

// randNormal returns a standard normal variate using Box-Muller.
func (pw *PriceWalk) randNormal() float64 {
	u1 := pw.rng.Float64()
	u2 := pw.rng.Float64()
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}
