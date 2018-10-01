package main

func main() {
	ReadConfig("traviota.yaml")
	log.Infof("Enabled sequences: %v", GetEnabledSeqNames())
	//p, err := GetEnabledSeqParams()
	//if err != nil{
	//	log.Critical(err)
	//	log.Info("Ciao")
	//	os.Exit(1)
	//} else {
	//	log.Infof("Enabled sequences: %v", p)
	//
	//}
}
