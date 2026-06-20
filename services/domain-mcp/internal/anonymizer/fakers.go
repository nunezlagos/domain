package anonymizer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

var firstNames = []string{
	"Ana", "Luis", "Sofía", "Mateo", "Camila", "Diego", "Valentina", "Pablo",
	"Isidora", "Joaquín", "Antonia", "Tomás", "Florencia", "Benjamín", "Catalina",
	"Vicente", "Emilia", "Agustín", "Constanza", "Maximiliano",
}

var lastNames = []string{
	"Gómez", "Soto", "Muñoz", "Rojas", "Pérez", "Fuentes", "Castro", "González",
	"López", "Vargas", "Silva", "Riquelme", "Espinoza", "Torres", "Núñez",
}

var orgWords = []string{
	"Atlas", "Pluma", "Vega", "Nodo", "Cima", "Coral", "Faro", "Sendero",
	"Brisa", "Cumbre", "Aurora", "Iris", "Andes", "Patagonia", "Pacífico",
}

// FakerEmail genera email determinístico estable.
func FakerEmail(seed int64, idx int) string {
	return fmt.Sprintf("user%d_%d@example.test", seed, idx)
}

// FakerName genera nombre+apellido determinístico.
func FakerName(seed int64, idx int) string {
	i := int(seed+int64(idx)) % len(firstNames)
	j := int(seed*7+int64(idx)) % len(lastNames)
	if i < 0 {
		i += len(firstNames)
	}
	if j < 0 {
		j += len(lastNames)
	}
	return firstNames[i] + " " + lastNames[j]
}

// FakerOrgName genera nombre de organización.
func FakerOrgName(seed int64, idx int) string {
	i := int(seed+int64(idx)) % len(orgWords)
	j := int(seed*3+int64(idx)) % len(orgWords)
	if i < 0 {
		i += len(orgWords)
	}
	if j < 0 {
		j += len(orgWords)
	}
	return orgWords[i] + " " + orgWords[j]
}

// FakerPhone genera teléfono chileno ficticio.
func FakerPhone(_ int64, idx int) string {
	return fmt.Sprintf("+5690000%04d", idx%10000)
}

// FakerRUT genera RUT chileno secuencial con DV módulo 11 válido.
// Rango: 10000000..99999999 → 90M valores únicos por seed.
func FakerRUT(seed int64, idx int) string {
	base := 10_000_000 + ((int64(idx) + seed*101) % 89_999_999)
	dv := rutDV(int(base))
	return fmt.Sprintf("%d-%s", base, dv)
}

// rutDV calcula el dígito verificador del RUT chileno (módulo 11).
// Retorna "0".."9" o "K".
func rutDV(n int) string {
	mult := []int{2, 3, 4, 5, 6, 7}
	sum, i := 0, 0
	for n > 0 {
		sum += (n % 10) * mult[i%len(mult)]
		n /= 10
		i++
	}
	r := 11 - (sum % 11)
	switch r {
	case 11:
		return "0"
	case 10:
		return "K"
	default:
		return fmt.Sprintf("%d", r)
	}
}

// RedactContentTag retorna el tag determinístico para content largo.
// Permite preservar uniqueness sin filtrar contenido real.
func RedactContentTag(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return "[REDACTED-" + hex.EncodeToString(h[:8]) + "]"
}
