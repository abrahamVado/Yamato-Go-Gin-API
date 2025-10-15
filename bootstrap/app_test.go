package bootstrap

import "testing"

//1.- TestSplitAndTrimNormalisesCommaSeparatedLists asserts the helper removes whitespace and empties.
func TestSplitAndTrimNormalisesCommaSeparatedLists(t *testing.T) {
        input := " https://one.test , ,https://two.test ,, "
        expected := []string{"https://one.test", "https://two.test"}

        result := splitAndTrim(input)
        if len(result) != len(expected) {
                t.Fatalf("expected %d entries, got %d", len(expected), len(result))
        }
        for index, value := range expected {
                if result[index] != value {
                        t.Fatalf("expected %s at index %d, got %s", value, index, result[index])
                }
        }
}
