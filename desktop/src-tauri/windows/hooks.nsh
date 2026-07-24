; Danmo Work Windows NSIS hooks — install bundled Microsoft Coreutils by default.
; Runs after files are copied into $INSTDIR so exec_shell has POSIX utilities without Git Bash.

!include LogicLib.nsh

!macro NSIS_HOOK_PREINSTALL
!macroend

!macro NSIS_HOOK_POSTINSTALL
  DetailPrint "Installing bundled Microsoft Coreutils for Windows..."
  StrCpy $R1 "$PROFILE\.dq-teams\bin\coreutils"
  CreateDirectory "$R1"
  CreateDirectory "$R1\bin"

  ; Tauri maps resources/coreutils/coreutils.exe → $INSTDIR\coreutils\coreutils.exe
  ${If} ${FileExists} "$INSTDIR\coreutils\coreutils.exe"
    CopyFiles /SILENT "$INSTDIR\coreutils\coreutils.exe" "$R1\coreutils.exe"
    ${If} ${FileExists} "$R1\coreutils.exe"
      ; Create applet hardlinks (ls.exe, cat.exe, grep.exe, …) next to the multi-call binary.
      nsExec::ExecToLog '"$R1\coreutils.exe" coreutils-manager refresh'
      Pop $R2
      ${If} ${FileExists} "$R1\bin\ls.exe"
        DetailPrint "Coreutils ready at $R1\bin (refresh exit $R2)"
      ${Else}
        DetailPrint "Coreutils refresh exit $R2 — hardlinks may be incomplete; app will retry on first launch"
      ${EndIf}
    ${Else}
      DetailPrint "WARNING: failed to copy coreutils.exe into $R1"
    ${EndIf}
  ${ElseIf} ${FileExists} "$INSTDIR\resources\coreutils\coreutils.exe"
    CopyFiles /SILENT "$INSTDIR\resources\coreutils\coreutils.exe" "$R1\coreutils.exe"
    nsExec::ExecToLog '"$R1\coreutils.exe" coreutils-manager refresh'
    Pop $R2
    DetailPrint "Coreutils installed from resources\ (refresh exit $R2)"
  ${Else}
    DetailPrint "WARNING: bundled coreutils.exe not found under $INSTDIR — Windows pack may be incomplete"
  ${EndIf}
!macroend

!macro NSIS_HOOK_PREUNINSTALL
!macroend

!macro NSIS_HOOK_POSTUNINSTALL
!macroend
