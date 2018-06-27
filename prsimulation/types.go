package main

type UPS struct {
    ID int
    PowerRating string
    PowerSupplyMethod string
    RunningState string
    SystemGrant string
    SystemType string
    DeviceTemperature string
    WorkMode string
    In Input
    Out Output
    Battery Battery
    Environment Environment
}

type Environment struct {
    Temperature string
    Humidty string
}

type Input struct {
    PowerFactor float64
    Frequency string
}

type Output struct {
    Voltage string
    Current string
    Crequerycy string
    PowerRating string
}

type Battery struct {
    State string
    Voltage string
    Current string
    Temperature string
    BackupTime string
    CapacityLeft string
}

type Refrigeration struct {
    // TODO
}
