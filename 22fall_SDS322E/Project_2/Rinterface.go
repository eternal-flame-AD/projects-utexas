//go:build cgo

package main

//go:generate ./genflags.sh

// #include <R.h>
// #include <Rinternals.h>
import "C"
import (
	"Project2/model"
	"log"
	"runtime"
	"unsafe"
)

type RNamed struct {
	Name string
	Val  C.SEXP
}

func rProtect(sexp C.SEXP) {
	C.Rf_protect(sexp)
}

func rUnprotect(n int) {
	C.Rf_unprotect(C.int(n))
}

func RPrint(sexp C.SEXP) {
	printStr := C.CString("print")
	defer C.free(unsafe.Pointer(printStr))
	print := C.Rf_install(printStr)
	printCall := C.Rf_lang2(print, sexp)
	C.Rf_protect(printCall)
	defer C.Rf_unprotect(1)

	errorOccurred := C.int(0)
	C.R_tryEval(printCall, C.R_GlobalEnv, &errorOccurred)
	if errorOccurred != 0 {
		log.Panicf("Error occurred in R_tryEval")
	}
}

func RDataFrame(list C.SEXP) C.SEXP {
	dataframeStr := C.CString("data.frame")
	defer C.free(unsafe.Pointer(dataframeStr))
	dataframe := C.Rf_install(dataframeStr)
	dfCall := C.Rf_lang2(dataframe, list)
	C.Rf_protect(dfCall)
	defer C.Rf_unprotect(1)

	errorOccurred := C.int(0)
	df := C.R_tryEval(dfCall, C.R_GlobalEnv, &errorOccurred)
	if errorOccurred != 0 {
		log.Panicf("Failed to create data frame")
	}
	return df
}

func MakeRList(kvs []model.KV) C.SEXP {
	var vals []RNamed
	for _, kv := range kvs {
		//log.Printf("Key[%d]: %s", i, kv.Key)
		switch v := kv.Value.(type) {
		case string:
			vals = append(vals, RNamed{kv.Key, RString([]string{v})})
		case int, int8, int16, int32, int64:
			vals = append(vals, RNamed{kv.Key, C.Rf_ScalarInteger(C.int(v.(int)))})
		case uint, uint8, uint16, uint32, uint64:
			vals = append(vals, RNamed{kv.Key, C.Rf_ScalarInteger(C.int(v.(int)))})
		case float32, float64:
			vals = append(vals, RNamed{kv.Key, C.Rf_ScalarReal(C.double(v.(float64)))})
		}
	}
	return RList(vals)
}

func RList(vals []RNamed) (ret C.SEXP) {
	if len(vals) > 5 {
		cStr := C.CString("c")
		defer C.free(unsafe.Pointer(cStr))
		c := C.Rf_install(cStr)

		cCall := C.Rf_lang3(c, RList(vals[:5]), RList(vals[5:]))
		C.Rf_protect(cCall)
		defer C.Rf_unprotect(1)

		errorOccurred := C.int(0)

		ret = C.R_tryEval(cCall, C.R_GlobalEnv, &errorOccurred)
		if errorOccurred != 0 {
			log.Panicf("Error occurred in R_tryEval")
		}
		return ret
	}
	listStr := C.CString("list")
	defer C.free(unsafe.Pointer(listStr))
	list := C.Rf_install(listStr)
	var listCall C.SEXP
	switch len(vals) {
	case 0:
		listCall = C.Rf_lang1(list)
	case 1:
		listCall = C.Rf_lang2(list, vals[0].Val)
	case 2:
		listCall = C.Rf_lang3(list, vals[0].Val, vals[1].Val)
	case 3:
		listCall = C.Rf_lang4(list, vals[0].Val, vals[1].Val, vals[2].Val)
	case 4:
		listCall = C.Rf_lang5(list, vals[0].Val, vals[1].Val, vals[2].Val, vals[3].Val)
	case 5:
		listCall = C.Rf_lang6(list, vals[0].Val, vals[1].Val, vals[2].Val, vals[3].Val, vals[4].Val)
	}
	C.Rf_protect(listCall)
	defer C.Rf_unprotect(1)

	listCallArgs := C.CDR(listCall)
	for _, val := range vals {
		if name := val.Name; name != "" {
			tmpStr := C.CString(name)
			defer C.free(unsafe.Pointer(tmpStr))
			nameSEXP := C.Rf_install(tmpStr)
			C.SET_TAG(listCallArgs, nameSEXP)
			listCallArgs = C.CDR(listCallArgs)
		}
	}

	errorOccurred := C.int(0)

	ret = C.R_tryEval(listCall, C.R_GlobalEnv, &errorOccurred)
	if errorOccurred != 0 {
		log.Panicf("Error occurred in R_tryEval")
	}

	return ret
}

func RRbind(left C.SEXP, right C.SEXP) (ret C.SEXP) {
	rbindCStr := C.CString("rbind")
	defer C.free(unsafe.Pointer(rbindCStr))
	rbind := C.Rf_install(rbindCStr)

	rbindCall := C.Rf_lang3(rbind, left, right)
	C.Rf_protect(rbindCall)
	defer C.Rf_unprotect(1)

	errorOccurred := C.int(0)

	ret = C.R_tryEval(rbindCall, C.R_GlobalEnv, &errorOccurred)
	if errorOccurred != 0 {
		log.Panicf("Error occurred in R_tryEval")
	}
	return ret
}

// RCBind calls cbind(left, name = val)
func RCBind(left C.SEXP, name string, val C.SEXP) (ret C.SEXP) {
	cbindCStr := C.CString("cbind")
	defer C.free(unsafe.Pointer(cbindCStr))
	cbind := C.Rf_install(cbindCStr)

	cbindCall := C.Rf_lang3(cbind, left, val)
	C.Rf_protect(cbindCall)
	defer C.Rf_unprotect(1)

	if name != "" {
		nameC := C.CString(name)
		defer C.free(unsafe.Pointer(nameC))
		nameSEXP := C.Rf_install(nameC)
		C.SET_TAG(C.CDR(C.CDR(cbindCall)), nameSEXP)
	}

	errorOccurred := C.int(0)

	ret = C.R_tryEval(cbindCall, C.R_GlobalEnv, &errorOccurred)
	if errorOccurred != 0 {
		log.Panicf("Error occurred in R_tryEval")
	}
	return ret
}

// GOString converts a R character vector to a Go string slice.
func GoString(input C.SEXP) []string {
	length := C.Rf_length(input)
	out := make([]string, length)
	for i := 0; i < int(length); i++ {
		out[i] = C.GoString(C.R_CHAR(C.STRING_ELT(input, C.R_xlen_t(i))))
	}
	return out
}

// RString converts a Go string slice to a R character vector.
func RString(input []string) C.SEXP {
	out := C.Rf_protect(C.allocVector(C.STRSXP, C.long(len(input))))

	for i, s := range input {
		tmpStr := C.CString(s)
		defer C.free(unsafe.Pointer(tmpStr))
		C.SET_STRING_ELT(out, C.R_xlen_t(i), C.mkChar(tmpStr))
	}

	C.Rf_unprotect(1)
	return out
}

//export ExtractPackages
func ExtractPackages(urlSexp C.SEXP, outputType C.SEXP, nProcsSexp C.SEXP) C.SEXP {
	nProcs := runtime.NumCPU()
	if nProcsSexp != C.R_NilValue {
		nProcs = int(C.asInteger(nProcsSexp))
	}
	urls := GoString(urlSexp)
	if nProcs < 1 {
		log.Panicf("Aborted: nProcs must be >= 1")
	}

	ret := extractPackages(urls, GoString(outputType)[0], nProcs)

	return RString(ret)
}
