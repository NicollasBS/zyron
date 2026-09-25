//go:build !windows
// +build !windows

package laser

type LaserController struct{}

func GetLaserController() *LaserController {
	return &LaserController{}
}

func (l *LaserController) UpdateLaser(xPercent, yPercent float64, active, click bool) {}

func (l *LaserController) HideLaser() {}
