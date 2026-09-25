package laser

import (
	"sync"
	"testing"
)

func TestLaserController_ThreadSafety(t *testing.T) {
	lc := GetLaserController()
	if lc == nil {
		t.Fatal("Esperava instância válida de LaserController, obteve nil")
	}

	// Testa chamadas concorrentes para garantir que não ocorram race conditions ou panics
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			x := float64(idx) / 20.0
			y := float64(idx) / 20.0
			lc.UpdateLaser(x, y, true, idx%2 == 0)
			lc.HideLaser()
		}(i)
	}
	wg.Wait()
}

func TestLaserController_CoordinatesBounds(t *testing.T) {
	lc := GetLaserController()
	// Testa valores extremos e fora dos limites para verificar robustez
	lc.UpdateLaser(-0.5, -0.5, true, false)
	lc.UpdateLaser(1.5, 1.5, true, false)
	lc.UpdateLaser(0.0, 0.0, true, true)
	lc.UpdateLaser(1.0, 1.0, false, false)
	lc.HideLaser()
}
