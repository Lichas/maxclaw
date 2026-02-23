!macro customHeader
  ; 自定义安装程序头部
!macroend

!macro preInit
  ; 初始化前执行
  ; 检查是否正在运行
  FindWindow $0 "Maxclaw" ""
  StrCmp $0 0 +3
    MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "Maxclaw is currently running. $
Please close it before continuing." IDOK preInit IDCANCEL quit
    quit:
      Quit
!macroend

!macro customInit
  ; 自定义初始化
!macroend

!macro customInstall
  ; 注册自定义协议 (maxclaw://)
  DetailPrint "Registering maxclaw:// protocol..."

  WriteRegStr HKCU "Software\\Classes\\maxclaw" "" "URL:Maxclaw Protocol"
  WriteRegStr HKCU "Software\\Classes\\maxclaw" "URL Protocol" ""
  WriteRegStr HKCU "Software\\Classes\\maxclaw\\DefaultIcon" "" "$INSTDIR\\${APP_EXECUTABLE_FILENAME}"
  WriteRegStr HKCU "Software\\Classes\\maxclaw\\shell\\open\\command" "" '"$INSTDIR\\${APP_EXECUTABLE_FILENAME}" "%1"'

  ; 添加到 PATH（可选，用于命令行使用）
  ; DetailPrint "Adding to PATH..."
  ; EnVar::AddValue "PATH" "$INSTDIR"

  ; 注册应用
  WriteRegStr HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\${UNINSTALL_REGISTRY_KEY}" "DisplayName" "Maxclaw AI Assistant"
  WriteRegStr HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\${UNINSTALL_REGISTRY_KEY}" "Publisher" "Lichas"
  WriteRegStr HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\${UNINSTALL_REGISTRY_KEY}" "HelpLink" "https://github.com/Lichas/maxclaw/issues"
  WriteRegStr HKCU "Software\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\${UNINSTALL_REGISTRY_KEY}" "URLInfoAbout" "https://github.com/Lichas/maxclaw"

  DetailPrint "Installation complete!"
!macroend

!macro customUnInstall
  ; 卸载时清理
  DetailPrint "Cleaning up..."

  ; 注销协议
  DeleteRegKey HKCU "Software\\Classes\\maxclaw"

  ; 从 PATH 移除
  ; EnVar::DeleteValue "PATH" "$INSTDIR"

  ; 删除用户数据（如果用户选择）
  MessageBox MB_YESNO|MB_ICONQUESTION "Do you want to remove all user data (settings, sessions, etc.)?$
$
This cannot be undone." IDYES removeData IDNO skipRemove

  removeData:
    DetailPrint "Removing user data..."
    RMDir /r "$LOCALAPPDATA\\maxclaw"
    RMDir /r "$APPDATA\\maxclaw"

  skipRemove:
    DetailPrint "Uninstall complete!"
!macroend

!macro customInstallMode
  ; 安装模式设置
  ; perUser 或 perMachine
!macroend
