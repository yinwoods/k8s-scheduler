package main

type UPS struct {
	ID                int    // 标识符
	PowerRating       int    // 额定功率
	PowerSupplyMethod int    // 供电方式
	RunningState      int    // 额定功率
	SystemGrant       int    // 机型容量
	SystemType        string // 系统型号
	WorkMode          int    // 工作模式
	In                Input
	Out               Output
	Battery           Battery
	Environment       Environment
}

type REF struct {
	ID                int    // 标识符
	PowerRating       int    // 额定功率
	PowerSupplyMethod int    // 供电方式
	RunningState      int    // 额定功率
	SystemGrant       int    // 机型容量
	SystemType        string // 系统型号
	WorkMode          int    // 工作模式
	In                Input
	Out               Output
	Battery           Battery
	Environment       Environment
}

type Environment struct {
	Temperature int // 环境温度
	Humidty     int // 环境湿度
}

type Input struct {
	PowerFactor int // 输入功率因数
	Frequency   int //  输入频率
}

type Output struct {
	Voltage     int // 电压
	Current     int // 电流
	Crequerycy  int // 输出频率
	PowerRating int // 额定功率
}

type Battery struct {
	State        string // 状态
	Voltage      int    // 电压
	Current      int    // 电流
	Temperature  int    // 温度
	BackupTime   int    // 后备时间
	CapacityLeft int    // 剩余容量
}
