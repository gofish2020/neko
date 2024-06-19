package main

import (
	"bytes"
	"embed"
	"image"
	_ "image/png"
	"io"
	"io/fs"
	"log"
	"math"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"crg.eti.br/go/config"
	_ "crg.eti.br/go/config/ini"
)

type neko struct {
	waiting  bool
	x        int
	y        int
	distance int // 鼠标和窗体的距离

	// 每种图片的两种状态显示（
	count int
	// 两种状态图片的切换频率
	min int // 8
	max int // 16

	// state 用于静态状态的精灵切换（当处于动态状态的时候，state会被一直赋值为0）
	state      int
	sprite     string // 精灵的形态（字符串），用来对应图片
	lastSprite string
	img        *ebiten.Image
}

type Config struct {
	Speed            int     `cfg:"speed" cfgDefault:"2" cfgHelper:"The speed of the cat."`
	Scale            float64 `cfg:"scale" cfgDefault:"2.0" cfgHelper:"The scale of the cat."`
	Quiet            bool    `cfg:"quiet" cfgDefault:"false" cfgHelper:"Disable sound."`
	MousePassthrough bool    `cfg:"mousepassthrough" cfgDefault:"false" cfgHelper:"Enable mouse passthrough."`
}

const (
	width  = 32
	height = 32
)

var (
	loaded  = false
	mSprite map[string]*ebiten.Image // 保存各种姿势的 精灵图片
	mSound  map[string][]byte

	//go:embed assets/*
	f embed.FS

	// 屏幕 宽高
	monitorWidth, monitorHeight = ebiten.Monitor().Size()

	cfg = &Config{}

	currentplayer *audio.Player = nil
)

func (m *neko) Layout(outsideWidth, outsideHeight int) (int, int) {
	return width, height
}

func playSound(sound []byte) {
	if cfg.Quiet {
		return
	}
	if currentplayer != nil && currentplayer.IsPlaying() {
		currentplayer.Close()
	}
	currentplayer = audio.CurrentContext().NewPlayerFromBytes(sound)
	currentplayer.SetVolume(.3)
	currentplayer.Play()
}

func (m *neko) Update() error {
	m.count++

	// 精灵处于静止状态 && 播放声音
	if m.state == 10 && m.count == m.min {
		playSound(mSound["idle3"])
	}
	// Prevents neko from being stuck on the side of the screen
	// or randomly travelling to another monitor
	m.x = max(0, min(m.x, monitorWidth))
	m.y = max(0, min(m.y, monitorHeight))

	// 这个其实在移动窗体的位置
	ebiten.SetWindowPosition(m.x, m.y)

	// 相当于鼠标相对于游戏窗口的中间点的位置信息
	mx, my := ebiten.CursorPosition()
	x := mx - (height / 2)
	y := my - (width / 2)

	dy, dx := y, x
	if dy < 0 {
		dy = -dy
	}
	if dx < 0 {
		dx = -dx
	}

	m.distance = dx + dy

	// 表示，鼠标和精灵在一起
	if m.distance < width || m.waiting {
		m.stayIdle() // 基于 m.state 的值，设定 m.sprite 的值
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			m.waiting = !m.waiting
		}
		return nil
	}

	// 表示鼠标离开了精灵的范围，并且如果当前处于 sleep的状态，发出醒来的声音
	if m.state >= 13 {
		playSound(mSound["awake"])
	}

	// 捕捉鼠标
	m.catchCursor(x, y)
	return nil
}

// 表示 小猫处于静止状态中...(静止状态：存在多种形态而已) 静止状态的时候，m.state取值处于[0,12]
func (m *neko) stayIdle() {
	// idle state
	switch m.state {
	case 0:
		m.state = 1
		fallthrough

	case 1, 2, 3: // 清醒
		m.sprite = "awake"

	case 4, 5, 6: // 挠痒痒（每个挠痒痒由2种图片）
		m.sprite = "scratch"

	case 7, 8, 9: // 洗澡
		m.sprite = "wash"

	case 10, 11, 12: // 哈欠
		m.min = 32
		m.max = 64
		m.sprite = "yawn"

	default: // 睡眠
		m.sprite = "sleep"
	}
}

// x/y表示鼠标的相对于窗体中点点坐标
// m.x/ m.y 表示窗体基于屏幕的坐标
func (m *neko) catchCursor(x, y int) {
	m.state = 0
	m.min = 8
	m.max = 16
	tr := 0.0
	// 求弧长 ->  比如 y = 1 x = 1 弧长即为 Pi/4 = 0.7853981633974483
	r := math.Atan2(float64(y), float64(x))
	if r <= 0 {
		tr = 360
	}

	// y/x =  tan 45度 = tan 45/180 * Pi = tan Pi/4
	// 弧长 -> 转角度
	a := (r / math.Pi * 180) + tr // Pi/4 / Pi * 180 = 45度

	// cfg.Speed 表示移动的速度 m.x m.y表示窗体的坐标

	// 相当于将 360 按照45一个范围，分割成8个区域
	switch {
	case a <= 292.5 && a > 247.5: // 上方
		m.y -= cfg.Speed
	case a <= 337.5 && a > 292.5: // 右上
		m.x += cfg.Speed
		m.y -= cfg.Speed
	case a <= 22.5 || a > 337.5: //  右边
		m.x += cfg.Speed
	case a <= 67.5 && a > 22.5: //   右下角
		m.x += cfg.Speed
		m.y += cfg.Speed
	case a <= 112.5 && a > 67.5: //  正下方向
		m.y += cfg.Speed
	case a <= 157.5 && a > 112.5: //  左下角
		m.x -= cfg.Speed
		m.y += cfg.Speed
	case a <= 202.5 && a > 157.5: //  左边
		m.x -= cfg.Speed
	case a <= 247.5 && a > 202.5: //  左上
		m.x -= cfg.Speed
		m.y -= cfg.Speed
	}

	switch {
	case a < 292 && a > 247:
		m.sprite = "up"
	case a < 337 && a > 292:
		m.sprite = "upright"
	case a < 22 || a > 337:
		m.sprite = "right"
	case a < 67 && a > 22:
		m.sprite = "downright"
	case a < 112 && a > 67:
		m.sprite = "down"
	case a < 157 && a > 112:
		m.sprite = "downleft"
	case a < 202 && a > 157:
		m.sprite = "left"
	case a < 247 && a > 202:
		m.sprite = "upleft"
	}
}

func (m *neko) Draw(screen *ebiten.Image) {
	var sprite string
	switch {
	//  awake 只有一个图片
	case m.sprite == "awake":
		sprite = m.sprite
	case m.count < m.min: // m.count 的作用，切换图片 1or2模式
		sprite = m.sprite + "1"
	default:
		sprite = m.sprite + "2"
	}

	m.img = mSprite[sprite]

	if m.count > m.max {
		m.count = 0 // 图片的切换频率 0-7 用图片1   8-17用图片2； 重新让 m.count从0 ++

		if m.state > 0 {
			m.state++
			switch m.state { // 如果鼠标和精灵不在一起， m.state 的值一直会被设置为0
			case 13: // m.state 能达到13，说明鼠标和精灵一直在一起（也就是鼠标没有移动），才有可能到达13
				playSound(mSound["sleep"])
			}
		}
	}

	// 同一个 精灵，不用重复渲染
	if m.lastSprite == sprite {
		return
	}

	m.lastSprite = sprite

	// 显示图片
	screen.Clear()
	screen.DrawImage(m.img, nil)
}

func main() {
	config.PrefixEnv = "NEKO"
	config.File = "neko.ini"
	config.Parse(cfg)

	mSprite = make(map[string]*ebiten.Image)
	mSound = make(map[string][]byte)

	// 读取目录
	a, _ := fs.ReadDir(f, "assets")
	for _, v := range a {
		// 读取文件内容
		data, _ := f.ReadFile("assets/" + v.Name())

		// 去掉文件名后缀（只剩下文件名）
		name := strings.TrimSuffix(v.Name(), filepath.Ext(v.Name()))
		// 文件后缀
		ext := filepath.Ext(v.Name())

		switch ext {
		case ".png":
			// 如果是图片，加载图片为 image.Image
			img, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				log.Fatal(err)
			}

			mSprite[name] = ebiten.NewImageFromImage(img) // 转换成 *ebiten.Image 类型
		case ".wav":
			// 如果是音频，读取 *wav.Stream
			stream, err := wav.DecodeWithSampleRate(44100, bytes.NewReader(data))
			if err != nil {
				log.Fatal(err)
			}
			// 这里的data 应该是pcm原始数据
			data, err := io.ReadAll(stream)
			if err != nil {
				log.Fatal(err)
			}

			mSound[name] = data
		}
	}

	audio.NewContext(44100)

	// Workaround: for some reason playing the first sound can incur significant delay.
	// So let's do this at the start.
	audio.CurrentContext().NewPlayerFromBytes([]byte{}).Play() // 类似于预热的感觉（可能是库有bug）

	n := &neko{
		x:   monitorWidth / 2,
		y:   monitorHeight / 2,
		min: 8,
		max: 16,
	}

	ebiten.SetRunnableOnUnfocused(true) // 游戏界面不现实，依然运行
	ebiten.SetScreenClearedEveryFrame(false)
	ebiten.SetTPS(50)
	ebiten.SetVsyncEnabled(true) // 垂直同步
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(true)                         // 置顶显示
	ebiten.SetWindowMousePassthrough(cfg.MousePassthrough) // 鼠标穿透
	ebiten.SetWindowSize(int(float64(width)*cfg.Scale), int(float64(height)*cfg.Scale))
	ebiten.SetWindowTitle("Neko")

	err := ebiten.RunGameWithOptions(n, &ebiten.RunGameOptions{
		InitUnfocused:     true, // 启动时候，窗体不聚焦
		ScreenTransparent: true, // 窗体透明
		SkipTaskbar:       true, // 图片不显示在任务栏
		X11ClassName:      "Neko",
		X11InstanceName:   "Neko",
	})
	if err != nil {
		log.Fatal(err)
	}
}
