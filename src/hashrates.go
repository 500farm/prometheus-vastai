package main

type HashRateMap map[string]float64

var HashRates HashRateMap = HashRateMap{ // MH/s
	// "A10": ,
	"A100 PCIE": 170,
	"A100 SXM4": 170,
	// "A40": ,
	"GTX 1070":    26,
	"GTX 1070 Ti": 28,
	"GTX 1080":    27,
	"GTX 1080 Ti": 36,
	// "GTX TITAN X": ,
	// "P106-100",
	// "Q RTX 4000": ,
	// "Q RTX 5000": ,
	// "Q RTX 6000": ,
	"Q RTX 8000": 50,
	// "RTX 2060": ,
	// "RTX 2060S": ,
	"RTX 2070":    36,
	"RTX 2070S":   37,
	"RTX 2080":    40,
	"RTX 2080 Ti": 47,
	// "RTX 2080S": ,
	// "RTX 3060": ,
	// "RTX 3060 Ti": ,
	"RTX 3070": 56,
	// "RTX 3070 Ti": ,
	"RTX 3080": 84,
	// "RTX 3080 Ti": ,
	"RTX 3090": 106,
	// "RTX A2000": ,
	// "RTX A4000": ,
	"RTX A5000": 87,
	"RTX A6000": 87,
	// "Tesla K80": ,
	"Tesla P100": 69,
	"Tesla T4":   25,
	"Tesla V100": 94,
	"Titan RTX":  74,
	"Titan Xp":   75,
}

/**
Non-LHR cards:
	RTX 3090	  Only Full Hash Rate
LHR-only cards:
	RTX 3080 Ti	  Only Lite Hash Rate
	RTX 3070 Ti	  Only Lite Hash Rate
Varying:
	RTX 3080	  Full Hash Rate or Lite Hash rate after late May 2021
	RTX 3070	  Full Hash Rate or Lite Hash rate after late May 2021
	RTX 3060 Ti	  Full Hash Rate or Lite Hash rate after late May 2021
	RTX 3060	  Lite Hash Rate or Full Hash Rate for v1 cards with leaked NVIDIA drivers

according to:
	https://www.nicehash.com/blog/post/how-to-distinguish-between-lhr-and-fhr-graphic-card?lang=en
*/
