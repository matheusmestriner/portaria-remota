package main

import "testing"

func TestNormalizePhone(t *testing.T) {
 phone,jid,err:=normalizePhone("+5511999999999")
 if err!=nil{t.Fatalf("unexpected error: %v",err)}
 if phone!="+5511999999999"{t.Fatalf("unexpected phone: %s",phone)}
 if jid.User!="5511999999999"{t.Fatalf("unexpected jid user: %s",jid.User)}
}

func TestNormalizePhoneRejectsInvalid(t *testing.T) {
 for _,value:=range []string{"","1199","abc","+05511999999999","55 11 99999-9999"}{
  if _,_,err:=normalizePhone(value);err==nil{t.Fatalf("expected %q to be rejected",value)}
 }
}
