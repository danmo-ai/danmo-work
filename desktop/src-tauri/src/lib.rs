use std::fs::{self, OpenOptions};
use std::io::Write;
use std::net::{SocketAddr, TcpStream, ToSocketAddrs};
use std::path::{Path, PathBuf};
use std::sync::Mutex;
use std::time::{Duration, Instant};
use tauri::{AppHandle, Emitter, Manager};
#[cfg(windows)]
use std::process::Command;
#[cfg(windows)]
use tauri::path::BaseDirectory;

const HELP_URL: &str = "https://github.com/danmo-ai/danmo-work#macos-%E5%AE%89%E8%A3%85";
const BACKEND_ADDR: &str = "127.0.0.1:7801";
const BACKEND_READY_TIMEOUT: Duration = Duration::from_secs(45);
const BACKEND_READY_INTERVAL: Duration = Duration::from_millis(200);

struct SidecarState {
    _child: Mutex<Option<std::process::Child>>,
}

/// Unified user data root: ~/.danmo-work (same as server/cli/tui).
fn teams_home(app: &AppHandle) -> Result<PathBuf, String> {
    let home = app
        .path()
        .home_dir()
        .map_err(|e| format!("failed to resolve home dir: {e}"))?;
    Ok(home.join(".danmo-work"))
}

fn find_sidecar_binary() -> Result<PathBuf, String> {
    let exe_dir = std::env::current_exe()
        .ok()
        .and_then(|p| p.parent().map(|d| d.to_path_buf()))
        .ok_or_else(|| "cannot determine exe directory".to_string())?;

    let triples = [
        "aarch64-apple-darwin",
        "x86_64-apple-darwin",
        "x86_64-unknown-linux-gnu",
        "x86_64-pc-windows-msvc",
    ];
    for triple in &triples {
        let candidate = exe_dir.join(format!("danmo-work-backend-{triple}"));
        if candidate.exists() {
            return Ok(candidate);
        }
    }
    Err(format!("sidecar binary not found in {}", exe_dir.display()))
}

/// Copy sidecar out of the .app bundle into ~/.danmo-work/bin.
/// macOS App Translocation / Gatekeeper often kills helpers launched from a
/// quarantined or translocated bundle path; running from the home data dir is stable.
fn prepare_runtime_binary(bundled: &Path, home: &Path) -> Result<PathBuf, String> {
    let bin_dir = home.join("bin");
    fs::create_dir_all(&bin_dir).map_err(|e| format!("create bin dir: {e}"))?;
    let runtime_bin = bin_dir.join("danmo-work-backend");
    let need_copy = match (fs::metadata(bundled), fs::metadata(&runtime_bin)) {
        (Ok(src), Ok(dst)) => {
            src.len() != dst.len()
                || src
                    .modified()
                    .ok()
                    .zip(dst.modified().ok())
                    .map(|(a, b)| a > b)
                    .unwrap_or(true)
        }
        (Ok(_), Err(_)) => true,
        (Err(e), _) => return Err(format!("sidecar metadata: {e}")),
    };
    if need_copy {
        fs::copy(bundled, &runtime_bin).map_err(|e| format!("copy sidecar: {e}"))?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mut perms = fs::metadata(&runtime_bin)
                .map_err(|e| format!("sidecar chmod stat: {e}"))?
                .permissions();
            perms.set_mode(0o755);
            fs::set_permissions(&runtime_bin, perms)
                .map_err(|e| format!("sidecar chmod: {e}"))?;
        }
    }
    Ok(runtime_bin)
}

/// Install bundled Microsoft Coreutils into ~/.danmo-work/bin/coreutils and create applet hardlinks.
/// Returns (coreutils.exe, bin_dir with ls.exe/…). Non-Windows builds return None.
fn prepare_coreutils(app: &AppHandle, home: &Path) -> Option<(PathBuf, PathBuf)> {
    #[cfg(not(windows))]
    {
        let _ = (app, home);
        return None;
    }
    #[cfg(windows)]
    {
        let bundled = app
            .path()
            .resolve("coreutils/coreutils.exe", BaseDirectory::Resource)
            .ok()
            .filter(|p| p.is_file())
            .or_else(|| {
                // Dev / unpackaged: next to sidecar or under src-tauri/resources.
                let exe_dir = std::env::current_exe()
                    .ok()
                    .and_then(|p| p.parent().map(|d| d.to_path_buf()))?;
                let candidates = [
                    exe_dir.join("coreutils").join("coreutils.exe"),
                    exe_dir
                        .join("resources")
                        .join("coreutils")
                        .join("coreutils.exe"),
                    exe_dir
                        .join("..")
                        .join("resources")
                        .join("coreutils")
                        .join("coreutils.exe"),
                ];
                candidates.into_iter().find(|p| p.is_file())
            })?;

        let root = home.join("bin").join("coreutils");
        let dst_exe = root.join("coreutils.exe");
        let bin_dir = root.join("bin");
        if let Err(e) = fs::create_dir_all(&bin_dir) {
            eprintln!("[coreutils] create dir: {e}");
            return None;
        }
        let need_copy = match (fs::metadata(&bundled), fs::metadata(&dst_exe)) {
            (Ok(src), Ok(dst)) => {
                src.len() != dst.len()
                    || src
                        .modified()
                        .ok()
                        .zip(dst.modified().ok())
                        .map(|(a, b)| a > b)
                        .unwrap_or(true)
            }
            (Ok(_), Err(_)) => true,
            (Err(e), _) => {
                eprintln!("[coreutils] metadata: {e}");
                return None;
            }
        };
        if need_copy {
            if let Err(e) = fs::copy(&bundled, &dst_exe) {
                eprintln!("[coreutils] copy: {e}");
                return None;
            }
        }
        // Prefer official hardlink sync when the binary supports coreutils-manager.
        let refreshed = Command::new(&dst_exe)
            .arg("coreutils-manager")
            .arg("refresh")
            .status()
            .map(|s| s.success())
            .unwrap_or(false);
        if !refreshed {
            // Fallback: create a minimal hardlink set so PATH at least has ls/cat/grep.
            for name in ["ls", "cat", "grep", "find", "xargs", "head", "tail", "wc", "cp", "mv", "rm", "mkdir", "touch", "sort", "uniq", "cut", "tr", "tee", "pwd", "echo", "printf", "env", "base64", "sha256sum", "md5sum", "realpath", "dirname", "basename", "mktemp", "sleep", "true", "false"] {
                let link = bin_dir.join(format!("{name}.exe"));
                if link.exists() {
                    continue;
                }
                if std::fs::hard_link(&dst_exe, &link).is_err() {
                    let _ = fs::copy(&dst_exe, &link);
                }
            }
        }
        if !bin_dir.join("ls.exe").is_file() {
            eprintln!("[coreutils] ls.exe missing after prepare");
            return None;
        }
        eprintln!(
            "[coreutils] ready: {} (bin: {})",
            dst_exe.display(),
            bin_dir.display()
        );
        Some((dst_exe, bin_dir))
    }
}

fn spawn_backend(app: &AppHandle) -> Result<(), String> {
    let home = teams_home(app)?;
    fs::create_dir_all(&home).map_err(|e| format!("failed to create ~/.danmo-work: {e}"))?;
    let work_dir = home.join("data");
    fs::create_dir_all(&work_dir).map_err(|e| format!("failed to create data dir: {e}"))?;

    let config_path = home.join("config.yaml");
    let log_path = home.join("backend.log");

    let bundled = find_sidecar_binary()?;
    let binary = prepare_runtime_binary(&bundled, &home)?;
    eprintln!("[sidecar] home: {}", home.display());
    eprintln!("[sidecar] runtime: {}", binary.display());

    let coreutils = prepare_coreutils(app, &home);

    let log_file = OpenOptions::new()
        .create(true)
        .append(true)
        .open(&log_path)
        .map_err(|e| format!("open backend log: {e}"))?;
    let log_err = log_file
        .try_clone()
        .map_err(|e| format!("clone backend log: {e}"))?;
    if let Ok(mut f) = OpenOptions::new().create(true).append(true).open(&log_path) {
        let _ = writeln!(f, "\n--- sidecar spawn ---");
    }

    let mut cmd = std::process::Command::new(&binary);
    cmd.current_dir(&home)
        .env("WORK_ADDR", BACKEND_ADDR)
        .env(
            "WORK_DB_PATH",
            home.join("work.db").to_string_lossy().as_ref(),
        )
        .env("WORK_CONFIG", config_path.to_string_lossy().as_ref())
        .env("WORK_DATA_DIR", work_dir.to_string_lossy().as_ref())
        .stdout(std::process::Stdio::from(log_file))
        .stderr(std::process::Stdio::from(log_err));
    if let Some((exe, bin)) = coreutils {
        cmd.env("WORK_COREUTILS_EXE", exe.to_string_lossy().as_ref());
        cmd.env("WORK_COREUTILS_BIN", bin.to_string_lossy().as_ref());
    }
    let mut child = cmd
        .spawn()
        .map_err(|e| format!("failed to spawn backend: {e}"))?;

    // Fail fast if the process exits immediately (common under App Translocation).
    std::thread::sleep(Duration::from_millis(400));
    match child.try_wait() {
        Ok(Some(status)) => {
            let tail = fs::read_to_string(&log_path).unwrap_or_default();
            let tail = tail.chars().rev().take(2000).collect::<String>();
            let tail: String = tail.chars().rev().collect();
            return Err(format!(
                "backend exited immediately ({status}). log tail:\n{tail}"
            ));
        }
        Ok(None) => {}
        Err(e) => return Err(format!("backend wait failed: {e}")),
    }

    app.manage(SidecarState {
        _child: Mutex::new(Some(child)),
    });

    // Emit ready only after the process is actually listening — spawn ≠ HTTP ready
    // (first launch may spend seconds on binary copy + SQLite migrate).
    let app_handle = app.clone();
    let log_for_ready = log_path.clone();
    std::thread::spawn(move || {
        if wait_for_backend_listen(BACKEND_ADDR, BACKEND_READY_TIMEOUT, BACKEND_READY_INTERVAL) {
            eprintln!("[sidecar] backend listening on {BACKEND_ADDR}");
            let _ = app_handle.emit("backend-ready", ());
        } else {
            let tail = fs::read_to_string(&log_for_ready).unwrap_or_default();
            let tail = tail.chars().rev().take(2000).collect::<String>();
            let tail: String = tail.chars().rev().collect();
            eprintln!(
                "[sidecar] backend did not become ready within {:?}. log tail:\n{tail}",
                BACKEND_READY_TIMEOUT
            );
            let _ = app_handle.emit("backend-failed", ());
        }
    });

    eprintln!(
        "[sidecar] backend spawned on {BACKEND_ADDR} (log: {})",
        log_path.display()
    );
    Ok(())
}

/// Poll until TCP connect succeeds (Gin listen), or timeout.
fn wait_for_backend_listen(addr: &str, timeout: Duration, interval: Duration) -> bool {
    let targets: Vec<SocketAddr> = match addr.to_socket_addrs() {
        Ok(iter) => iter.collect(),
        Err(e) => {
            eprintln!("[sidecar] resolve {addr}: {e}");
            return false;
        }
    };
    if targets.is_empty() {
        return false;
    }
    let deadline = Instant::now() + timeout;
    while Instant::now() < deadline {
        for target in &targets {
            if TcpStream::connect_timeout(target, Duration::from_millis(150)).is_ok() {
                return true;
            }
        }
        std::thread::sleep(interval);
    }
    false
}

/// Open help documentation on first launch (macOS only, due to unsigned app)
fn handle_first_launch(app: &AppHandle) {
    #[cfg(target_os = "macos")]
    {
        if let Ok(home) = teams_home(app) {
            let marker = home.join(".first_launch_done");
            if !marker.exists() {
                let _ = fs::create_dir_all(&home);
                let _ = fs::write(&marker, "1");
                let _ = open::that(HELP_URL);
            }
        }
    }
}

#[tauri::command]
fn open_external(url: String) -> Result<(), String> {
    open::that(&url).map_err(|e| format!("failed to open: {e}"))
}

/// Write bytes to a path chosen via the save dialog (turn log zip, etc.).
#[tauri::command]
fn write_file_bytes(path: String, contents: Vec<u8>) -> Result<(), String> {
    if let Some(parent) = Path::new(&path).parent() {
        if !parent.as_os_str().is_empty() {
            fs::create_dir_all(parent).map_err(|e| format!("create parent dir: {e}"))?;
        }
    }
    fs::write(&path, contents).map_err(|e| format!("write {path}: {e}"))
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .invoke_handler(tauri::generate_handler![open_external, write_file_bytes])
        .setup(|app| {
            handle_first_launch(&app.handle());
            if let Err(e) = spawn_backend(&app.handle()) {
                eprintln!("WARNING: backend sidecar failed to start: {e}");
                eprintln!("The app will run without backend API. Start it manually if needed.");
            }
            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
