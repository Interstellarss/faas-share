package binpack2d

import (
  "fmt"
  "math/rand"
  "testing"
)

func TestPacker (t *testing.T) {
  const numRects = 20
  packer := Create(64, 64)

  for rule := 0; rule < 5; rule++ {
    r := rand.New(rand.NewSource(1234)) // use fixed randomized values
    accepted := 0
    fmt.Printf("Packing with rule %d...\n", rule)
    for i := 0; i < numRects; i++ {
      w, h := ((r.Int() % 4) + 1) * 4, ((r.Int() % 4) + 1) * 4
      rect, ok := packer.Insert(w, h, rule)
      if ok {
        accepted++
        fmt.Printf("Accepted: %v\n", rect)
      } else {
        fmt.Printf("Rejected: {%d %d}\n", w, h)
      }
    }
    packer.ShrinkBin(false)
    fmt.Printf("Accepted rects: %d out of %d (efficiency: %d%%). Final bin size: %d x %d\n\n", accepted, numRects, int(packer.GetOccupancy() * 100), packer.GetWidth(), packer.GetHeight())
    packer.Clear()
  }
}
