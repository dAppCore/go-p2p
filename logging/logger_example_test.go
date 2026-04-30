package logging

import core "dappco.re/go"

func ExampleLevel_String() {

	_ = Level.String

	core.Println("Level.String")

	// Output: Level.String

}

func ExampleDefaultConfig() {

	_ = DefaultConfig

	core.Println("DefaultConfig")

	// Output: DefaultConfig

}

func ExampleNew() {

	_ = New

	core.Println("New")

	// Output: New

}

func ExampleLogger_WithComponent() {

	_ = (*Logger).WithComponent

	core.Println("Logger.WithComponent")

	// Output: Logger.WithComponent

}

func ExampleLogger_SetLevel() {

	_ = (*Logger).SetLevel

	core.Println("Logger.SetLevel")

	// Output: Logger.SetLevel

}

func ExampleLogger_GetLevel() {

	_ = (*Logger).GetLevel

	core.Println("Logger.GetLevel")

	// Output: Logger.GetLevel

}

func ExampleLogger_Debug() {

	_ = (*Logger).Debug

	core.Println("Logger.Debug")

	// Output: Logger.Debug

}

func ExampleLogger_Info() {

	_ = (*Logger).Info

	core.Println("Logger.Info")

	// Output: Logger.Info

}

func ExampleLogger_Warn() {

	_ = (*Logger).Warn

	core.Println("Logger.Warn")

	// Output: Logger.Warn

}

func ExampleLogger_Error() {

	_ = (*Logger).Error

	core.Println("Logger.Error")

	// Output: Logger.Error

}

func ExampleLogger_Debugf() {

	_ = (*Logger).Debugf

	core.Println("Logger.Debugf")

	// Output: Logger.Debugf

}

func ExampleLogger_Infof() {

	_ = (*Logger).Infof

	core.Println("Logger.Infof")

	// Output: Logger.Infof

}

func ExampleLogger_Warnf() {

	_ = (*Logger).Warnf

	core.Println("Logger.Warnf")

	// Output: Logger.Warnf

}

func ExampleLogger_Errorf() {

	_ = (*Logger).Errorf

	core.Println("Logger.Errorf")

	// Output: Logger.Errorf

}

func ExampleSetGlobal() {

	_ = SetGlobal

	core.Println("SetGlobal")

	// Output: SetGlobal

}

func ExampleGetGlobal() {

	_ = GetGlobal

	core.Println("GetGlobal")

	// Output: GetGlobal

}

func ExampleSetGlobalLevel() {

	_ = SetGlobalLevel

	core.Println("SetGlobalLevel")

	// Output: SetGlobalLevel

}

func ExampleDebug() {

	_ = Debug

	core.Println("Debug")

	// Output: Debug

}

func ExampleInfo() {

	_ = Info

	core.Println("Info")

	// Output: Info

}

func ExampleWarn() {

	_ = Warn

	core.Println("Warn")

	// Output: Warn

}

func ExampleError() {

	_ = Error

	core.Println("Error")

	// Output: Error

}

func ExampleDebugf() {

	_ = Debugf

	core.Println("Debugf")

	// Output: Debugf

}

func ExampleInfof() {

	_ = Infof

	core.Println("Infof")

	// Output: Infof

}

func ExampleWarnf() {

	_ = Warnf

	core.Println("Warnf")

	// Output: Warnf

}

func ExampleErrorf() {

	_ = Errorf

	core.Println("Errorf")

	// Output: Errorf

}

func ExampleParseLevel() {

	_ = ParseLevel

	core.Println("ParseLevel")

	// Output: ParseLevel

}
