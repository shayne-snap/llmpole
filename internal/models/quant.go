// Package models provides the model database and quantization helpers.
package models

// QuantHierarchy lists quantizations from best quality to most compressed (used for best-quant selection).
var QuantHierarchy = []string{"Q8_0", "Q6_K", "Q5_K_M", "Q4_K_M", "Q3_K_M", "Q2_K"}

// QuantBPP returns bytes per parameter for the given quantization.
func QuantBPP(quant string) float64 {
	switch quant {
	case "F32":
		return 4.0
	case "F16", "BF16":
		return 2.0
	case "Q8_0":
		return 1.05
	case "Q6_K":
		return 0.80
	case "Q5_K_M":
		return 0.68
	case "Q4_K_M", "Q4_0":
		return 0.58
	case "Q3_K_M":
		return 0.48
	case "Q2_K":
		return 0.37
	default:
		return 0.58
	}
}

// QuantSpeedMultiplier returns the relative inference speed factor for the quantization.
func QuantSpeedMultiplier(quant string) float64 {
	switch quant {
	case "F16", "BF16":
		return 0.6
	case "Q8_0":
		return 0.8
	case "Q6_K":
		return 0.95
	case "Q5_K_M":
		return 1.0
	case "Q4_K_M", "Q4_0":
		return 1.15
	case "Q3_K_M":
		return 1.25
	case "Q2_K":
		return 1.35
	default:
		return 1.0
	}
}

// QuantQualityPenalty returns the quality score penalty for the quantization (used in scoring).
func QuantQualityPenalty(quant string) float64 {
	switch quant {
	case "F16", "BF16", "Q8_0":
		return 0.0
	case "Q6_K":
		return -1.0
	case "Q5_K_M":
		return -2.0
	case "Q4_K_M", "Q4_0":
		return -5.0
	case "Q3_K_M":
		return -8.0
	case "Q2_K":
		return -12.0
	default:
		return -5.0
	}
}
