package nativeapp

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Display-only transformation. Original text, choices, provenance and saved
// learning records are never rewritten. Treat an ordered 1,2,... run as a list,
// rather than interpreting individual I/V variables as enumeration markers.
var stemMarkerRE = regexp.MustCompile(`[IVXLCDM]+[.．、:：)）]|[ⅠⅡⅢⅣⅤⅥⅦⅧⅨⅩⅪⅫⅰⅱⅲⅳⅴⅵⅶⅷⅸⅹⅺⅻ]+[.．、:：)）]?|[①-⑳]`)
var stemProtectedRE = regexp.MustCompile("(?s)```.*?```|~~~.*?~~~|`[^`]*`|" + `\\\(.*?\\\)|\\\[.*?\\\]|\$\$.*?\$\$|\$[^$\n]*\$|(?i:<table\b[^>]*>.*?</table>)|!?\[[^\]]*\]\([^)]*\)|<[^>]*>`)
var stemRomans = []string{"I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X", "XI", "XII", "XIII", "XIV", "XV", "XVI", "XVII", "XVIII", "XIX", "XX"}

type stemMarker struct {
	at, value int
	family    string
}

func formatStemItems(src string) string {
	protected := stemProtectedRE.FindAllStringIndex(src, -1)
	matches := stemMarkerRE.FindAllStringIndex(src, -1)
	var markers []stemMarker
	p := 0
	for _, match := range matches {
		for p < len(protected) && protected[p][1] <= match[0] {
			p++
		}
		if p < len(protected) && protected[p][0] < match[1] {
			continue
		}
		token := src[match[0]:match[1]]
		first, _ := utf8.DecodeRuneInString(token)
		marker := stemMarker{at: match[0], family: "roman"}
		if first >= '①' && first <= '⑳' {
			marker.value = int(first-'①') + 1
			marker.family = "circled"
		} else {
			if match[0] > 0 {
				before, width := utf8.DecodeLastRuneInString(src[:match[0]])
				if before == '（' || before == '(' {
					marker.at -= width
				}
				if !unicode.IsSpace(before) && !strings.ContainsRune("。；;：:，,！？!?（）()", before) {
					continue
				}
			}
			token = strings.TrimRight(token, ".．、:：)）")
			var canonical strings.Builder
			for _, r := range token {
				switch {
				case r >= 'Ⅰ' && r <= 'Ⅻ':
					canonical.WriteString(stemRomans[int(r-'Ⅰ')])
				case r >= 'ⅰ' && r <= 'ⅻ':
					canonical.WriteString(stemRomans[int(r-'ⅰ')])
				default:
					canonical.WriteRune(r)
				}
			}
			for i, r := range stemRomans {
				if r == canonical.String() {
					marker.value = i + 1
					break
				}
			}
		}
		if marker.value > 0 {
			markers = append(markers, marker)
		}
	}
	// Require at least two ordered items of the same notation. References to
	// numerals in prose or an answer combination are left untouched.
	var breaks []int
	for i := 0; i < len(markers); i++ {
		if markers[i].value != 1 {
			continue
		}
		run := []int{markers[i].at}
		expected := 2
		j := i + 1
		for ; j < len(markers); j++ {
			m := markers[j]
			if m.family != markers[i].family || m.value != expected {
				break
			}
			run = append(run, m.at)
			expected++
		}
		if len(run) > 1 {
			breaks = append(breaks, run...)
			i = j - 1
		}
	}
	if len(breaks) == 0 {
		return src
	}
	var out strings.Builder
	last := 0
	for _, at := range breaks {
		part := strings.TrimRightFunc(src[last:at], unicode.IsSpace)
		out.WriteString(part)
		if out.Len() > 0 {
			out.WriteString("\n\n")
		}
		last = at
	}
	out.WriteString(src[last:])
	return out.String()
}
