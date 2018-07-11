package main

import (
	"math/rand"
	"time"
)

func randRange(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}

func randString(str ...string) string {
	rand.Seed(time.Now().UnixNano())
	return str[rand.Intn(len(str))]
}

func randomUPS() *UPS {
	rand.Seed(time.Now().UnixNano())
	return &UPS{
		ID:                randRange(1, 5),
		PowerRating:       randRange(1, 9),
		PowerSupplyMethod: randRange(1, 7),
		RunningState:      randRange(1, 5),
		SystemGrant:       randRange(1, 100000),
		SystemType:        randString("UPS5000", "UPS8000"),
		WorkMode:          rand.Intn(4),
		In: Input{
			PowerFactor: randRange(-100, 100),
			Frequency:   randRange(0, 100),
		},
		Out: Output{
			Voltage:     randRange(0, 10000),
			Current:     randRange(0, 10000),
			Crequerycy:  randRange(0, 100),
			PowerRating: randRange(1, 9),
		},
		Battery: Battery{
			State:        randString("PowerNormal", "PowerLow"),
			Voltage:      randRange(0, 10000),
			Current:      randRange(0, 10000),
			Temperature:  randRange(-20, 80),
			BackupTime:   randRange(0, 172800),
			CapacityLeft: randRange(0, 100),
		},
		Environment: Environment{
			Temperature: randRange(-20, 80),
			Humidty:     randRange(0, 1000),
		},
	}
}

func randomREF() *REF {
	rand.Seed(time.Now().UnixNano())
	return &REF{
		ID:                randRange(1, 5),
		PowerRating:       randRange(1, 9),
		PowerSupplyMethod: randRange(1, 7),
		RunningState:      randRange(1, 5),
		SystemGrant:       randRange(1, 100000),
		SystemType:        randString("REF5300", "REF6600"),
		WorkMode:          rand.Intn(4),
		In: Input{
			PowerFactor: randRange(-100, 100),
			Frequency:   randRange(0, 100),
		},
		Out: Output{
			Voltage:     randRange(0, 10000),
			Current:     randRange(0, 10000),
			Crequerycy:  randRange(0, 100),
			PowerRating: randRange(1, 9),
		},
		Battery: Battery{
			State:        randString("PowerNormal", "PowerLow"),
			Voltage:      randRange(0, 10000),
			Current:      randRange(0, 10000),
			Temperature:  randRange(-20, 80),
			BackupTime:   randRange(0, 172800),
			CapacityLeft: randRange(0, 100),
		},
		Environment: Environment{
			Temperature: randRange(-20, 80),
			Humidty:     randRange(0, 1000),
		},
	}
}
