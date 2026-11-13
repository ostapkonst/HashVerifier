; =============================================================================
; HashVerifier — Inno Setup Script
; =============================================================================
; Cross-platform checksum generation and verification tool
; =============================================================================

#define MyAppName "HashVerifier"
#define MyAppPublisher "Ostap Konstantinov"
#define MyAppURL "https://github.com/ostapkonst/HashVerifier"
#define MyAppExeName "hashverifier.exe"

; Version passed from command line: /DAppVersion=1.0.0
#ifndef AppVersion
  #error AppVersion is not set. Pass /DAppVersion=X.X.X to the compiler.
#endif

; Architecture passed from command line: /DAppArch=amd64 or /DAppArch=i686
#ifndef AppArch
  #define AppArch "amd64"
#endif

; Source directory passed from command line: /DSourceDir=../dist/windows-amd64
; When running in Docker, this should be set to /work (the mounted directory)
#ifndef SourceDir
  #define SourceDir "."
#endif

[Setup]
; Application identity
AppId={{49028601-93D0-4249-B2C5-ACAA48680F84}
AppName={#MyAppName}
AppVersion={#AppVersion}
AppVerName={#MyAppName} {#AppVersion} ({#AppArch})
UninstallDisplayName={#MyAppName} ({#AppArch})
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}

; License file
LicenseFile=hashverifier-license.txt

; Installation paths
DefaultDirName={autopf}\{#MyAppName}
DefaultGroupName={#MyAppName}
AllowNoIcons=yes

; Architecture detection
#if AppArch == "amd64"
  ArchitecturesAllowed=x64compatible
  ArchitecturesInstallIn64BitMode=x64compatible
#else
  ArchitecturesAllowed=x86
#endif

; Output
OutputDir=output
OutputBaseFilename=hashverifier-{#AppVersion}-windows-{#AppArch}
SetupIconFile=favicon.ico
UninstallDisplayIcon={app}\{#MyAppExeName}

; File version information
VersionInfoVersion={#AppVersion}
VersionInfoCompany={#MyAppPublisher}
VersionInfoDescription={#MyAppName} Installer
VersionInfoCopyright=Copyright © 2026 {#MyAppPublisher}

; Compression
Compression=lzma2/ultra64
SolidCompression=yes
LZMAUseSeparateProcess=yes

; Wizard style
WizardStyle=modern
WizardSizePercent=100,100

; Privileges
; The installer asks whether to install for all users (elevation) or for the current
; user only (no UAC). A previous installation fixes the mode: upgrades never ask again.
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
UsePreviousPrivileges=yes

; Minimum Windows version
MinVersion=6.1sp1

ChangesAssociations=yes

; Prevent multiple instances
SetupMutex=HashVerifierSetupMutex

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "sendtoicon"; Description: "Add to Send-To menu"; GroupDescription: "Shell Integration:"; Check: not IsAdminInstallMode
Name: "assoc_sfv"; Description: ".sfv (CRC32)"; GroupDescription: "File Associations:"
Name: "assoc_md4"; Description: ".md4 (MD4)"; GroupDescription: "File Associations:"
Name: "assoc_md5"; Description: ".md5 (MD5)"; GroupDescription: "File Associations:"
Name: "assoc_sha1"; Description: ".sha1 (SHA1)"; GroupDescription: "File Associations:"
Name: "assoc_sha256"; Description: ".sha256 (SHA256)"; GroupDescription: "File Associations:"
Name: "assoc_sha384"; Description: ".sha384 (SHA384)"; GroupDescription: "File Associations:"
Name: "assoc_sha512"; Description: ".sha512 (SHA512)"; GroupDescription: "File Associations:"
Name: "assoc_sha3_256"; Description: ".sha3-256 (SHA3-256)"; GroupDescription: "File Associations:"
Name: "assoc_sha3_384"; Description: ".sha3-384 (SHA3-384)"; GroupDescription: "File Associations:"
Name: "assoc_sha3_512"; Description: ".sha3-512 (SHA3-512)"; GroupDescription: "File Associations:"
Name: "assoc_blake3"; Description: ".blake3 (BLAKE3)"; GroupDescription: "File Associations:"
Name: "assoc_xxh3"; Description: ".xxh3 (XXH3)"; GroupDescription: "File Associations:"
Name: "assoc_xxh128"; Description: ".xxh128 (XXH128)"; GroupDescription: "File Associations:"

[Files]
; Main executable
Source: "{#SourceDir}\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion

; GTK3 and runtime DLLs
Source: "{#SourceDir}\*.dll"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs

; GTK3 configuration
Source: "{#SourceDir}\etc\*"; DestDir: "{app}\etc"; Flags: ignoreversion recursesubdirs

; GTK3 schemas
Source: "{#SourceDir}\share\glib-2.0\schemas\*"; DestDir: "{app}\share\glib-2.0\schemas"; Flags: ignoreversion recursesubdirs

; GTK3 icons
Source: "{#SourceDir}\share\icons\*"; DestDir: "{app}\share\icons"; Flags: ignoreversion recursesubdirs

; GTK3 themes (may be empty)
Source: "{#SourceDir}\share\themes\*"; DestDir: "{app}\share\themes"; Flags: ignoreversion recursesubdirs skipifsourcedoesntexist

; Documentation
Source: "{#SourceDir}\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#SourceDir}\THIRD_PARTY_NOTICES"; DestDir: "{app}"; Flags: ignoreversion

; File type icon for associations (installed but not shown in UI)
Source: "hashverifier-filetype.ico"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; Start Menu (auto = all users or current user, matching the install mode)
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autoprograms}\{cm:UninstallProgram,{#MyAppName}}"; Filename: "{uninstallexe}"

; Desktop (auto = all users or current user, matching the install mode)
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

; Send-To menu (per-user only; hidden in administrative install mode)
Name: "{usersendto}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: sendtoicon; Check: not IsAdminInstallMode

[Registry]
; File associations — SFV
Root: HKA; Subkey: "Software\Classes\.sfv"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SFV"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_sfv
Root: HKA; Subkey: "Software\Classes\HashVerifier.SFV"; ValueType: string; ValueName: ""; ValueData: "SFV Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sfv
Root: HKA; Subkey: "Software\Classes\HashVerifier.SFV\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sfv
Root: HKA; Subkey: "Software\Classes\HashVerifier.SFV\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sfv

; File associations — MD4
Root: HKA; Subkey: "Software\Classes\.md4"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.MD4"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_md4
Root: HKA; Subkey: "Software\Classes\HashVerifier.MD4"; ValueType: string; ValueName: ""; ValueData: "MD4 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_md4
Root: HKA; Subkey: "Software\Classes\HashVerifier.MD4\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_md4
Root: HKA; Subkey: "Software\Classes\HashVerifier.MD4\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_md4

; File associations — MD5
Root: HKA; Subkey: "Software\Classes\.md5"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.MD5"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_md5
Root: HKA; Subkey: "Software\Classes\HashVerifier.MD5"; ValueType: string; ValueName: ""; ValueData: "MD5 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_md5
Root: HKA; Subkey: "Software\Classes\HashVerifier.MD5\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_md5
Root: HKA; Subkey: "Software\Classes\HashVerifier.MD5\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_md5

; File associations — SHA1
Root: HKA; Subkey: "Software\Classes\.sha1"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA1"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_sha1
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA1"; ValueType: string; ValueName: ""; ValueData: "SHA1 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha1
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA1\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha1
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA1\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha1

; File associations — SHA256
Root: HKA; Subkey: "Software\Classes\.sha256"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA256"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_sha256
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA256"; ValueType: string; ValueName: ""; ValueData: "SHA256 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha256
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA256\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha256
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA256\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha256

; File associations — SHA384
Root: HKA; Subkey: "Software\Classes\.sha384"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA384"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_sha384
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA384"; ValueType: string; ValueName: ""; ValueData: "SHA384 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha384
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA384\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha384
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA384\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha384

; File associations — SHA512
Root: HKA; Subkey: "Software\Classes\.sha512"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA512"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_sha512
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA512"; ValueType: string; ValueName: ""; ValueData: "SHA512 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha512
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA512\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha512
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA512\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha512

; File associations — SHA3-256
Root: HKA; Subkey: "Software\Classes\.sha3-256"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA3_256"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_sha3_256
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA3_256"; ValueType: string; ValueName: ""; ValueData: "SHA3-256 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha3_256
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA3_256\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha3_256
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA3_256\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha3_256

; File associations — SHA3-384
Root: HKA; Subkey: "Software\Classes\.sha3-384"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA3_384"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_sha3_384
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA3_384"; ValueType: string; ValueName: ""; ValueData: "SHA3-384 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha3_384
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA3_384\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha3_384
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA3_384\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha3_384

; File associations — SHA3-512
Root: HKA; Subkey: "Software\Classes\.sha3-512"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.SHA3_512"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_sha3_512
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA3_512"; ValueType: string; ValueName: ""; ValueData: "SHA3-512 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_sha3_512
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA3_512\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_sha3_512
Root: HKA; Subkey: "Software\Classes\HashVerifier.SHA3_512\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_sha3_512

; File associations — BLAKE3
Root: HKA; Subkey: "Software\Classes\.blake3"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.BLAKE3"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_blake3
Root: HKA; Subkey: "Software\Classes\HashVerifier.BLAKE3"; ValueType: string; ValueName: ""; ValueData: "BLAKE3 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_blake3
Root: HKA; Subkey: "Software\Classes\HashVerifier.BLAKE3\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_blake3
Root: HKA; Subkey: "Software\Classes\HashVerifier.BLAKE3\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_blake3

; File associations — XXH3
Root: HKA; Subkey: "Software\Classes\.xxh3"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.XXH3"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_xxh3
Root: HKA; Subkey: "Software\Classes\HashVerifier.XXH3"; ValueType: string; ValueName: ""; ValueData: "XXH3 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_xxh3
Root: HKA; Subkey: "Software\Classes\HashVerifier.XXH3\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_xxh3
Root: HKA; Subkey: "Software\Classes\HashVerifier.XXH3\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_xxh3

; File associations — XXH128
Root: HKA; Subkey: "Software\Classes\.xxh128"; ValueType: string; ValueName: ""; ValueData: "HashVerifier.XXH128"; Flags: uninsdeletevalue uninsdeletekeyifempty; Tasks: assoc_xxh128
Root: HKA; Subkey: "Software\Classes\HashVerifier.XXH128"; ValueType: string; ValueName: ""; ValueData: "XXH128 Checksum File"; Flags: uninsdeletekey; Tasks: assoc_xxh128
Root: HKA; Subkey: "Software\Classes\HashVerifier.XXH128\DefaultIcon"; ValueType: string; ValueName: ""; ValueData: "{app}\hashverifier-filetype.ico"; Tasks: assoc_xxh128
Root: HKA; Subkey: "Software\Classes\HashVerifier.XXH128\shell\open\command"; ValueType: string; ValueName: ""; ValueData: """{app}\{#MyAppExeName}"" ""%1"""; Tasks: assoc_xxh128

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent unchecked

[Code]
const
  UninstallKey = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\{49028601-93D0-4249-B2C5-ACAA48680F84}_is1';

{ The architecture is part of UninstallDisplayName, so an existing installation's
  DisplayName tells whether the other architecture is already present. }
function ReadInstalledArch(RootKey: Integer; var Arch: String): Boolean;
var
  DisplayName: String;
begin
  Arch := '';
  Result := RegQueryStringValue(RootKey, UninstallKey, 'DisplayName', DisplayName);
  if Result then
  begin
    if Pos('(i686)', DisplayName) > 0 then
      Arch := 'i686'
    else if Pos('(amd64)', DisplayName) > 0 then
      Arch := 'amd64';
  end;
end;

function CheckArch(RootKey: Integer): Boolean;
var
  Arch: String;
begin
  Result := True;
  if ReadInstalledArch(RootKey, Arch) and (Arch <> '') and (Arch <> '{#AppArch}') then
  begin
    MsgBox('HashVerifier (' + Arch + ') is already installed. Uninstall it before installing the {#AppArch} version.', mbError, MB_OK);
    Result := False;
  end;
end;

function AdminInstalled: Boolean;
var
  Arch: String;
begin
  Result := ReadInstalledArch(HKLM32, Arch);
  if not Result then
    if IsWin64 then
      Result := ReadInstalledArch(HKLM64, Arch);
end;

function UserInstalled: Boolean;
var
  Arch: String;
begin
  Result := ReadInstalledArch(HKCU, Arch);
  if not Result then
    if IsWin64 then
      Result := ReadInstalledArch(HKCU64, Arch);
end;

{ Two modes cannot coexist, and a second architecture is refused: the uninstall
  entry cannot be shared safely and the per-user directory is the same for both. }
function InitializeSetup: Boolean;
begin
  Result := True;

  if AdminInstalled and UserInstalled then
  begin
    MsgBox('HashVerifier is installed both for all users and for the current user. Uninstall both HashVerifier entries in Settings > Apps, then run Setup again.', mbError, MB_OK);
    Result := False;
    Exit;
  end;

  if not CheckArch(HKLM32) then
  begin
    Result := False;
    Exit;
  end;

  if not CheckArch(HKCU) then
  begin
    Result := False;
    Exit;
  end;

  if IsWin64 then
  begin
    if not CheckArch(HKLM64) then
    begin
      Result := False;
      Exit;
    end;

    if not CheckArch(HKCU64) then
    begin
      Result := False;
      Exit;
    end;
  end;
end;

{ UsePreviousPrivileges already suppresses the mode dialog for an upgrade;
  this catches an explicit /ALLUSERS or /CURRENTUSER override instead. }
function PrepareToInstall(var NeedsRestart: Boolean): String;
begin
  Result := '';
  if IsAdminInstallMode then
  begin
    if UserInstalled then
      Result := 'HashVerifier is already installed for the current user. Uninstall it before installing for all users.';
  end
  else if AdminInstalled then
    Result := 'HashVerifier is already installed for all users. Uninstall it before installing for the current user, or choose "Install for all users".';
end;

procedure InitializeWizard;
begin
  WizardForm.LicenseMemo.WordWrap := False;
  WizardForm.LicenseMemo.ScrollBars := ssBoth;
end;
