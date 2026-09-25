//go:build windows
// +build windows

package laser

import (
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	WS_EX_TOPMOST     = 0x00000008
	WS_EX_TRANSPARENT = 0x00000020
	WS_EX_TOOLWINDOW  = 0x00000080
	WS_EX_LAYERED     = 0x00080000
	WS_EX_NOACTIVATE  = 0x08000000

	WS_POPUP   = 0x80000000
	WS_VISIBLE = 0x10000000

	HWND_TOPMOST = ^uintptr(0)

	SWP_NOACTIVATE = 0x0010
	SWP_SHOWWINDOW = 0x0040

	LWA_COLORKEY = 0x00000001

	SM_CXSCREEN = 0
	SM_CYSCREEN = 1

	SW_HIDE           = 0
	SW_SHOWNOACTIVATE = 4

	WM_DESTROY = 0x0002
	WM_PAINT   = 0x000F
	WM_TIMER   = 0x0113
	WM_USER    = 0x0400

	WM_LASER_UPDATE = WM_USER + 1
	WM_LASER_HIDE   = WM_USER + 2
	WM_LASER_CLICK  = WM_USER + 3

	laserSize = 80 // Dimensão da janela do laser em pixels
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	gdi32  = windows.NewLazySystemDLL("gdi32.dll")

	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procGetSystemMetrics           = user32.NewProc("GetSystemMetrics")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procSetTimer                   = user32.NewProc("SetTimer")
	procKillTimer                  = user32.NewProc("KillTimer")
	procFillRect                   = user32.NewProc("FillRect")

	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreatePen        = gdi32.NewProc("CreatePen")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procEllipse          = gdi32.NewProc("Ellipse")
)

type WNDCLASSEX struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type PAINTSTRUCT struct {
	hdc         uintptr
	fErase      int32
	rcPaint     RECT
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

type POINT struct {
	X, Y int32
}

type MSG struct {
	Hwnd    uintptr
	Message uint32
	Wparam  uintptr
	Lparam  uintptr
	Time    uint32
	Pt      POINT
}

// LaserController gerencia a janela do ponteiro laser no Windows
type LaserController struct {
	hwnd        uintptr
	initialized bool
	mu          sync.Mutex
	isClicking  bool
	lastUpdate  time.Time
}

var (
	globalLaser   *LaserController
	laserInitOnce sync.Once
)

func GetLaserController() *LaserController {
	laserInitOnce.Do(func() {
		globalLaser = &LaserController{}
		go globalLaser.initWindowThread()
	})
	return globalLaser
}

func (l *LaserController) initWindowThread() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	className, _ := windows.UTF16PtrFromString("ZyronLaserWindowClass")
	windowName, _ := windows.UTF16PtrFromString("ZyronLaserPointer")

	wc := WNDCLASSEX{
		cbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		style:         0,
		lpfnWndProc:   syscall.NewCallback(laserWndProc),
		hInstance:     0,
		lpszClassName: className,
	}

	ret, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		return
	}

	hwnd, _, _ := procCreateWindowExW.Call(
		WS_EX_TOPMOST|WS_EX_LAYERED|WS_EX_TRANSPARENT|WS_EX_TOOLWINDOW|WS_EX_NOACTIVATE,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		WS_POPUP,
		uintptr(0x80000000), // CW_USEDEFAULT
		uintptr(0x80000000),
		uintptr(laserSize),
		uintptr(laserSize),
		0, 0, 0, 0,
	)

	if hwnd == 0 {
		return
	}

	// Cor-chave para transparência total: Magenta RGB(255, 0, 255)
	// Todo pixel Magenta torna-se 100% transparente e clica-se através dele
	colorkey := uint32(0x00FF00FF)
	procSetLayeredWindowAttributes.Call(hwnd, uintptr(colorkey), 255, LWA_COLORKEY)

	l.mu.Lock()
	l.hwnd = hwnd
	l.initialized = true
	l.mu.Unlock()

	var msg MSG
	for {
		res, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if res == 0 || int32(res) == -1 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func laserWndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			drawLaser(hdc, globalLaser != nil && globalLaser.isClicking)
			procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		}
		return 0

	case WM_LASER_UPDATE:
		// LParam empacota X (16 bits baixos) e Y (16 bits altos)
		x := int32(int16(lparam & 0xFFFF))
		y := int32(int16((lparam >> 16) & 0xFFFF))
		posX := int(x) - (laserSize / 2)
		posY := int(y) - (laserSize / 2)

		procSetWindowPos.Call(
			hwnd,
			HWND_TOPMOST,
			uintptr(posX),
			uintptr(posY),
			uintptr(laserSize),
			uintptr(laserSize),
			SWP_NOACTIVATE|SWP_SHOWWINDOW,
		)
		procInvalidateRect.Call(hwnd, 0, 1)

		// Reseta timer de auto-hide (2.5 segundos)
		procSetTimer.Call(hwnd, 1, 2500, 0)
		return 0

	case WM_LASER_CLICK:
		if globalLaser != nil {
			globalLaser.isClicking = true
			procInvalidateRect.Call(hwnd, 0, 1)
			// Timer para encerrar animação do pulso do clique (350ms)
			procSetTimer.Call(hwnd, 2, 350, 0)
		}
		return 0

	case WM_LASER_HIDE:
		procShowWindow.Call(hwnd, SW_HIDE)
		return 0

	case WM_TIMER:
		timerID := wparam
		if timerID == 1 {
			// Auto-hide por inatividade
			procKillTimer.Call(hwnd, 1)
			procShowWindow.Call(hwnd, SW_HIDE)
		} else if timerID == 2 {
			// Fim do pulso de clique
			procKillTimer.Call(hwnd, 2)
			if globalLaser != nil {
				globalLaser.isClicking = false
			}
			procInvalidateRect.Call(hwnd, 0, 1)
		}
		return 0

	case WM_DESTROY:
		return 0
	}

	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return ret
}

// drawLaser desenha o ponteiro laser com efeito de brilho e ondas
func drawLaser(hdc uintptr, isClick bool) {
	// 1. Fundo Magenta transparente
	magentaBrush, _, _ := procCreateSolidBrush.Call(0x00FF00FF)
	bgRect := RECT{Left: 0, Top: 0, Right: laserSize, Bottom: laserSize}
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&bgRect)), magentaBrush)
	procDeleteObject.Call(magentaBrush)

	center := int32(laserSize / 2)

	// 2. Se for clique, desenha onda expansiva amarela/vermelha pulsante
	if isClick {
		ripplePen, _, _ := procCreatePen.Call(0, 3, 0x0000FFFF) // Amarelo vibrante
		oldPen, _, _ := procSelectObject.Call(hdc, ripplePen)

		// Pincel nulo (apenas contorno)
		nullBrush, _, _ := gdi32.NewProc("GetStockObject").Call(5) // NULL_BRUSH
		oldBrush, _, _ := procSelectObject.Call(hdc, nullBrush)

		rippleRadius := int32(34)
		procEllipse.Call(hdc,
			uintptr(center-rippleRadius),
			uintptr(center-rippleRadius),
			uintptr(center+rippleRadius),
			uintptr(center+rippleRadius),
		)

		procSelectObject.Call(hdc, oldPen)
		procSelectObject.Call(hdc, oldBrush)
		procDeleteObject.Call(ripplePen)
	}

	// 3. Halo / Anel Externo Vermelho Brilhante
	outerPen, _, _ := procCreatePen.Call(0, 2, 0x003333FF) // Vermelho vivo
	oldPen, _, _ := procSelectObject.Call(hdc, outerPen)

	nullBrush, _, _ := gdi32.NewProc("GetStockObject").Call(5)
	oldBrush, _, _ := procSelectObject.Call(hdc, nullBrush)

	outerRadius := int32(20)
	procEllipse.Call(hdc,
		uintptr(center-outerRadius),
		uintptr(center-outerRadius),
		uintptr(center+outerRadius),
		uintptr(center+outerRadius),
	)

	// 4. Linhas de mira / cruz sutil nos quatro eixos
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(outerPen)

	// 5. Núcleo Interno Sólido (Vermelho Intenso)
	coreBrush, _, _ := procCreateSolidBrush.Call(0x000000FF) // RGB(255, 0, 0)
	corePen, _, _ := procCreatePen.Call(0, 1, 0x0000FFFF)    // Borda amarela sutil
	procSelectObject.Call(hdc, coreBrush)
	procSelectObject.Call(hdc, corePen)

	coreRadius := int32(8)
	procEllipse.Call(hdc,
		uintptr(center-coreRadius),
		uintptr(center-coreRadius),
		uintptr(center+coreRadius),
		uintptr(center+coreRadius),
	)

	// 6. Ponto Central de Luz (Branco/Amarelo puro)
	centerBrush, _, _ := procCreateSolidBrush.Call(0x00FFFFFF)
	centerPen, _, _ := procCreatePen.Call(0, 1, 0x00FFFFFF)
	procSelectObject.Call(hdc, centerBrush)
	procSelectObject.Call(hdc, centerPen)

	centerDot := int32(3)
	procEllipse.Call(hdc,
		uintptr(center-centerDot),
		uintptr(center-centerDot),
		uintptr(center+centerDot),
		uintptr(center+centerDot),
	)

	// Limpeza de recursos GDI
	procSelectObject.Call(hdc, oldBrush)
	procSelectObject.Call(hdc, oldPen)
	procDeleteObject.Call(coreBrush)
	procDeleteObject.Call(corePen)
	procDeleteObject.Call(centerBrush)
	procDeleteObject.Call(centerPen)
}

// UpdateLaser posiciona o ponteiro laser proporcionalmente à tela do cliente
func (l *LaserController) UpdateLaser(xPercent, yPercent float64, active, click bool) {
	l.mu.Lock()
	if !l.initialized || l.hwnd == 0 {
		l.mu.Unlock()
		return
	}
	hwnd := l.hwnd
	l.mu.Unlock()

	if !active {
		procPostMessageW.Call(hwnd, WM_LASER_HIDE, 0, 0)
		return
	}

	screenW, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	screenH, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)

	if screenW == 0 {
		screenW = 1920
	}
	if screenH == 0 {
		screenH = 1080
	}

	targetX := int32(xPercent * float64(screenW))
	targetY := int32(yPercent * float64(screenH))

	if targetX < 0 {
		targetX = 0
	}
	if targetY < 0 {
		targetY = 0
	}

	lparam := uintptr(uint32(uint16(targetX)) | (uint32(uint16(targetY)) << 16))
	procPostMessageW.Call(hwnd, WM_LASER_UPDATE, 0, lparam)

	if click {
		procPostMessageW.Call(hwnd, WM_LASER_CLICK, 0, 0)
	}
}

// HideLaser oculta imediatamente o ponteiro laser
func (l *LaserController) HideLaser() {
	l.mu.Lock()
	if !l.initialized || l.hwnd == 0 {
		l.mu.Unlock()
		return
	}
	hwnd := l.hwnd
	l.mu.Unlock()

	procPostMessageW.Call(hwnd, WM_LASER_HIDE, 0, 0)
}
