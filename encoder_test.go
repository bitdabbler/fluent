package fluent

import (
	"bytes"
	"testing"
)

func TestEncoder_DeepCopyRaw(t *testing.T) {
	e1 := NewEncoder(defaultNewBufferCap)
	if err := e1.EncodeString("this is just a random test string"); err != nil {
		t.Fatal(err)
	}
	e2 := e1.DeepCopy()
	if *e1 == *e2 {
		t.Fatal("DeepCopy points to same value as the original")
	}
	if !bytes.Equal(e1.Bytes(), e2.Bytes()) {
		t.Fatalf("*Encoder and it's deep copy do not have idential byte arrays, e1.Len() = %d, e2.Len() = %d", e1.Len(), e2.Len())
	}
}

func TestEncoder_DeepCopyPooled(t *testing.T) {
	p, err := NewEncoderPool("test-tag", nil)
	if err != nil {
		t.Fatal(err)
	}
	e1 := p.Get()
	if err = e1.EncodeString("this is just a random test string"); err != nil {
		t.Fatal(err)
	}
	e2 := e1.DeepCopy()
	if *e1 == *e2 {
		t.Fatal("DeepCopy points to same value as the original")
	}
	if !bytes.Equal(e1.Bytes(), e2.Bytes()) {
		t.Fatalf("*Encoder and it's deep copy do not have idential byte arrays, e1.Len() = %d, e2.Len() = %d", e1.Len(), e2.Len())
	}
}
