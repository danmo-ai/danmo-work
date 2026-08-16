use std::fs::{self, OpenOptions};
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpStream, ToSocketAddrs};
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Mutex;
use std::time::{Duration, Instant};
use tauri::{AppHandle, Emitter, Manager};
use tauri::path::BaseDirectory;

const HELP_URL: &str = "https://github.com/danmo-ai/danmo-work#macos-%E5%AE%89%E8%A3%85";
const BACKEND_ADDR: &str = "127.0.0.1:7801";
const BACKEND_READY_TIMEOUT: Duration = Duration::from_secs(45);
const BACKEND_READY_INTERVAL: Duration = Duration::from_millis(200);
const BACKEND_PID_FILE: &str = "backend.pid";

/// Owns the desktop sidecar. Rust's `Child` drop does **not** kill the process,
/// so we must terminate explicitly on app exit / Drop / uninstall.
struct SidecarState {
    child: Mutex<Option<std::process::Child>>,
    home: PathBuf,
    stopped: AtomicBool,
}

impl SidecarState {
    fn shutdown(&self) {
        if self
            .stopped
            .compare_exchange(false, true, Ordering::SeqCst, Ordering::SeqCst)
            .is_err()
        {
            return;
        }
        let mut guard = match self.child.lock() {
            Ok(g) => g,
            Err(p) => p.into_inner(),
        };
        if let Some(mut child) = guard.take() {
            terminate_child(&mut child);
        }
        let pid_path = self.home.join(BACKEND_PID_FILE);
        if let Ok(raw) = fs::read_to_string(&pid_path) {
            if let Ok(pid) = raw.trim().parse::<u32>() {
                eprintln!("[sidecar] exit: stopping pidfile backend pid={pid}");
                stop_pid(pid);
            }
        }
        let _ = fs::remove_file(&pid_path);
        eprintln!("[sidecar] shutdown complete");
    }
}

impl Drop for SidecarState {
    fn drop(&mut self) {
        self.shutdown();
    }
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

    // Packaged macOS/Windows keep the Rust target triple on externalBin;
    // Linux AppImage/.deb strip it to plain danmo-work-backend next to the exe.
    let names = [
        "danmo-work-backend-aarch64-apple-darwin",
        "danmo-work-backend-x86_64-apple-darwin",
        "danmo-work-backend-x86_64-unknown-linux-gnu",
        "danmo-work-backend-x86_64-pc-windows-msvc.exe",
        "danmo-work-backend-x86_64-pc-windows-msvc",
        "danmo-work-backend",
        "danmo-work-backend.exe",
    ];
    for name in &names {
        let candidate = exe_dir.join(name);
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
    let stamp_path = bin_dir.join("danmo-work-backend.stamp");
    let src_meta = fs::metadata(bundled).map_err(|e| format!("sidecar metadata: {e}"))?;
    let src_mtime = src_meta
        .modified()
        .ok()
        .and_then(|t| t.duration_since(std::time::UNIX_EPOCH).ok())
        .map(|d| d.as_secs())
        .unwrap_or(0);
    let wanted_stamp = format!("{}:{}", src_meta.len(), src_mtime);
    let have_stamp = fs::read_to_string(&stamp_path).unwrap_or_default();
    let need_copy = have_stamp.trim() != wanted_stamp || !runtime_bin.exists();
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
        let _ = fs::write(&stamp_path, &wanted_stamp);
        eprintln!("[sidecar] refreshed runtime binary ({wanted_stamp})");
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

fn external_backend_requested() -> bool {
    // `make dev-desktop` starts Go via scripts, then launches Tauri. Without this
    // gate, setup() reclaim+spawn kills the script backend and races itself.
    for key in ["WORK_EXTERNAL_BACKEND", "SKIP_BACKEND"] {
        match std::env::var(key).as_deref() {
            Ok("1") | Ok("true") | Ok("TRUE") | Ok("yes") | Ok("YES") => return true,
            _ => {}
        }
    }
    false
}

fn spawn_backend(app: &AppHandle) -> Result<(), String> {
    let home = teams_home(app)?;
    fs::create_dir_all(&home).map_err(|e| format!("failed to create ~/.danmo-work: {e}"))?;
    let work_dir = home.join("data");
    fs::create_dir_all(&work_dir).map_err(|e| format!("failed to create data dir: {e}"))?;

    let config_path = home.join("config.yaml");
    let log_path = home.join("backend.log");
    let pid_path = home.join(BACKEND_PID_FILE);

    if external_backend_requested() {
        eprintln!(
            "[sidecar] WORK_EXTERNAL_BACKEND/SKIP_BACKEND set — not reclaiming or spawning; expecting API on {BACKEND_ADDR}"
        );
        if let Ok(mut f) = OpenOptions::new().create(true).append(true).open(&log_path) {
            let _ = writeln!(
                f,
                "\n--- external backend mode (skip sidecar spawn) {BACKEND_ADDR} ---"
            );
        }
        // UI polls /api/v1/version; emit ready if already up so boot is instant.
        if backend_version_ok(BACKEND_ADDR) {
            let _ = app.emit("backend-ready", ());
        }
        return Ok(());
    }

    // Stale listeners on :7801 (old sidecar / `make backend`) keep answering the UI
    // after upgrades — users still see WeChat QR 404 and SQLite DBMOVED (1032).
    reclaim_backend_port(&pid_path, &log_path);

    let bundled = find_sidecar_binary()?;
    let binary = prepare_runtime_binary(&bundled, &home)?;
    eprintln!("[sidecar] home: {}", home.display());
    eprintln!("[sidecar] runtime: {}", binary.display());

    let coreutils = prepare_coreutils(app, &home);
    prepare_first_launch_script(app, &home);

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
        let _ = writeln!(f, "binary={}", binary.display());
    }

    let mut cmd = Command::new(&binary);
    cmd.current_dir(&home)
        .env("WORK_ADDR", BACKEND_ADDR)
        .env(
            "WORK_DB_PATH",
            home.join("work.db").to_string_lossy().as_ref(),
        )
        .env(
            "WORK_STORE_DB_PATH",
            home.join("store.db").to_string_lossy().as_ref(),
        )
        .env("WORK_CONFIG", config_path.to_string_lossy().as_ref())
        .env("WORK_DATA_DIR", work_dir.to_string_lossy().as_ref())
        .stdout(std::process::Stdio::from(log_file))
        .stderr(std::process::Stdio::from(log_err));
    // Own process group on Unix so Exit/Drop can SIGTERM/SIGKILL the whole tree
    // (Go may spawn helpers; killing only the parent leaves orphans).
    #[cfg(unix)]
    {
        use std::os::unix::process::CommandExt;
        cmd.process_group(0);
    }
    if let Some((exe, bin)) = coreutils {
        cmd.env("WORK_COREUTILS_EXE", exe.to_string_lossy().as_ref());
        cmd.env("WORK_COREUTILS_BIN", bin.to_string_lossy().as_ref());
    }
    let mut child = cmd
        .spawn()
        .map_err(|e| format!("failed to spawn backend: {e}"))?;

    if let Err(e) = fs::write(&pid_path, child.id().to_string()) {
        eprintln!("[sidecar] write pid file: {e}");
    }

    // Fail fast if the process exits immediately (common under App Translocation).
    std::thread::sleep(Duration::from_millis(400));
    match child.try_wait() {
        Ok(Some(status)) => {
            let _ = fs::remove_file(&pid_path);
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

    let child_pid = child.id();
    app.manage(SidecarState {
        child: Mutex::new(Some(child)),
        home: home.clone(),
        stopped: AtomicBool::new(false),
    });

    // Emit ready only after OUR process is listening — not a foreign leftover on :7801.
    let app_handle = app.clone();
    let log_for_ready = log_path.clone();
    let pid_for_ready = pid_path.clone();
    std::thread::spawn(move || {
        if wait_for_our_backend(
            BACKEND_ADDR,
            child_pid,
            BACKEND_READY_TIMEOUT,
            BACKEND_READY_INTERVAL,
        ) {
            eprintln!("[sidecar] backend pid={child_pid} listening on {BACKEND_ADDR}");
            let _ = app_handle.emit("backend-ready", ());
        } else {
            let _ = fs::remove_file(&pid_for_ready);
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
        "[sidecar] backend spawned pid={child_pid} on {BACKEND_ADDR} (log: {})",
        log_path.display()
    );
    Ok(())
}

/// Stop a previous desktop sidecar and anything still listening on BACKEND_ADDR.
fn reclaim_backend_port(pid_path: &Path, log_path: &Path) {
    if let Ok(raw) = fs::read_to_string(pid_path) {
        if let Ok(pid) = raw.trim().parse::<u32>() {
            eprintln!("[sidecar] stopping previous backend pid={pid}");
            stop_pid(pid);
        }
        let _ = fs::remove_file(pid_path);
    }
    for pid in listeners_on_backend_port() {
        eprintln!("[sidecar] reclaiming {BACKEND_ADDR} from pid={pid}");
        stop_pid(pid);
    }
    // Give the OS a moment to release the port / SQLite locks.
    std::thread::sleep(Duration::from_millis(300));
    if let Ok(mut f) = OpenOptions::new().create(true).append(true).open(log_path) {
        let _ = writeln!(f, "--- reclaim {BACKEND_ADDR} ---");
    }
}

fn stop_pid(pid: u32) {
    if pid == 0 {
        return;
    }
    #[cfg(unix)]
    {
        // Negative PID = process group (when spawned with process_group(0)).
        let pg = format!("-{pid}");
        let _ = Command::new("kill").args(["-TERM", &pg]).status();
        let _ = Command::new("kill").args(["-TERM", &pid.to_string()]).status();
        std::thread::sleep(Duration::from_millis(200));
        let _ = Command::new("kill").args(["-KILL", &pg]).status();
        let _ = Command::new("kill").args(["-KILL", &pid.to_string()]).status();
    }
    #[cfg(windows)]
    {
        let _ = Command::new("taskkill")
            .args(["/PID", &pid.to_string(), "/T", "/F"])
            .status();
    }
}

/// Graceful-then-force terminate of a spawned sidecar Child.
fn terminate_child(child: &mut std::process::Child) {
    let pid = child.id();
    eprintln!("[sidecar] terminating backend pid={pid}");
    #[cfg(unix)]
    {
        let pg = format!("-{pid}");
        let _ = Command::new("kill").args(["-TERM", &pg]).status();
        let _ = Command::new("kill").args(["-TERM", &pid.to_string()]).status();
        std::thread::sleep(Duration::from_millis(250));
        match child.try_wait() {
            Ok(Some(status)) => {
                eprintln!("[sidecar] backend pid={pid} exited ({status})");
                return;
            }
            _ => {
                let _ = Command::new("kill").args(["-KILL", &pg]).status();
                let _ = child.kill();
                let _ = child.wait();
                eprintln!("[sidecar] backend pid={pid} force-killed");
            }
        }
    }
    #[cfg(windows)]
    {
        let _ = Command::new("taskkill")
            .args(["/PID", &pid.to_string(), "/T", "/F"])
            .status();
        let _ = child.wait();
        eprintln!("[sidecar] backend pid={pid} taskkilled");
    }
    #[cfg(not(any(unix, windows)))]
    {
        let _ = child.kill();
        let _ = child.wait();
    }
}

/// Called on ExitRequested / Exit — Child drop alone never kills the OS process.
fn shutdown_sidecar(app: &AppHandle) {
    if let Some(state) = app.try_state::<SidecarState>() {
        state.shutdown();
        return;
    }
    // Spawn failed before manage(), but a pidfile may still exist.
    if let Ok(home) = teams_home(app) {
        let pid_path = home.join(BACKEND_PID_FILE);
        if let Ok(raw) = fs::read_to_string(&pid_path) {
            if let Ok(pid) = raw.trim().parse::<u32>() {
                eprintln!("[sidecar] exit: stopping orphan pidfile backend pid={pid}");
                stop_pid(pid);
            }
        }
        let _ = fs::remove_file(&pid_path);
    }
}

fn listeners_on_backend_port() -> Vec<u32> {
    #[cfg(unix)]
    {
        let output = Command::new("lsof")
            .args(["-nP", "-iTCP:7801", "-sTCP:LISTEN", "-t"])
            .output();
        let Ok(out) = output else {
            return Vec::new();
        };
        String::from_utf8_lossy(&out.stdout)
            .lines()
            .filter_map(|line| line.trim().parse::<u32>().ok())
            .collect()
    }
    #[cfg(windows)]
    {
        let output = Command::new("netstat").args(["-ano"]).output();
        let Ok(out) = output else {
            return Vec::new();
        };
        let mut pids = Vec::new();
        for line in String::from_utf8_lossy(&out.stdout).lines() {
            let lower = line.to_ascii_lowercase();
            if !(lower.contains("7801") && lower.contains("listen")) {
                continue;
            }
            if let Some(pid) = line.split_whitespace().last().and_then(|s| s.parse().ok()) {
                if pid > 0 && !pids.contains(&pid) {
                    pids.push(pid);
                }
            }
        }
        pids
    }
    #[cfg(not(any(unix, windows)))]
    {
        Vec::new()
    }
}

/// True when BACKEND_ADDR answers and (best-effort) is our spawned pid.
fn wait_for_our_backend(
    addr: &str,
    child_pid: u32,
    timeout: Duration,
    interval: Duration,
) -> bool {
    let deadline = Instant::now() + timeout;
    while Instant::now() < deadline {
        if !wait_for_backend_listen(addr, Duration::from_millis(100), Duration::from_millis(20)) {
            std::thread::sleep(interval);
            continue;
        }
        // Prefer verifying the listener belongs to our child (avoids ready-on-stale).
        let listeners = listeners_on_backend_port();
        if listeners.is_empty() || listeners.contains(&child_pid) {
            if backend_version_ok(addr) {
                return true;
            }
        } else {
            eprintln!(
                "[sidecar] {addr} held by {:?}, expected pid={child_pid}; reclaiming",
                listeners
            );
            for pid in listeners {
                if pid != child_pid {
                    stop_pid(pid);
                }
            }
        }
        std::thread::sleep(interval);
    }
    false
}

fn backend_version_ok(addr: &str) -> bool {
    // Minimal HTTP/1.0 GET — enough to ensure Gin (not a random TCP service) answers.
    let Ok(mut stream) = TcpStream::connect(addr) else {
        return false;
    };
    let _ = stream.set_read_timeout(Some(Duration::from_millis(500)));
    let _ = stream.set_write_timeout(Some(Duration::from_millis(500)));
    let req = "GET /api/v1/version HTTP/1.0\r\nHost: 127.0.0.1\r\nConnection: close\r\n\r\n";
    if stream.write_all(req.as_bytes()).is_err() {
        return false;
    }
    let mut buf = Vec::new();
    let _ = stream.read_to_end(&mut buf);
    let body = String::from_utf8_lossy(&buf);
    body.contains("\"version\"") || body.contains("HTTP/1.0 200") || body.contains("HTTP/1.1 200")
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

/// Copy the platform first-launch script from the app bundle into ~/.danmo-work/first_launch.
/// The Go sidecar runs it asynchronously on startup (optional post-install hooks).
fn prepare_first_launch_script(app: &AppHandle, home: &Path) {
    let dest_dir = home.join("first_launch");
    if let Err(e) = fs::create_dir_all(&dest_dir) {
        eprintln!("[first-launch] create dir: {e}");
        return;
    }

    #[cfg(windows)]
    let names = ["post-install.ps1", "PLATFORM"];
    #[cfg(not(windows))]
    let names = ["post-install.sh", "PLATFORM"];

    for name in names {
        let bundled = resolve_first_launch_resource(app, name);
        let Some(src) = bundled else {
            continue;
        };
        let dst = dest_dir.join(name);
        let need_copy = match (fs::metadata(&src), fs::metadata(&dst)) {
            (Ok(a), Ok(b)) => {
                a.len() != b.len()
                    || a.modified()
                        .ok()
                        .zip(b.modified().ok())
                        .map(|(x, y)| x > y)
                        .unwrap_or(true)
            }
            (Ok(_), Err(_)) => true,
            (Err(e), _) => {
                eprintln!("[first-launch] metadata {}: {e}", src.display());
                continue;
            }
        };
        if need_copy {
            if let Err(e) = fs::copy(&src, &dst) {
                eprintln!("[first-launch] copy {}: {e}", name);
                continue;
            }
            #[cfg(unix)]
            if name.ends_with(".sh") {
                use std::os::unix::fs::PermissionsExt;
                if let Ok(meta) = fs::metadata(&dst) {
                    let mut perms = meta.permissions();
                    perms.set_mode(0o755);
                    let _ = fs::set_permissions(&dst, perms);
                }
            }
            eprintln!("[first-launch] staged {}", dst.display());
        }
    }
}

fn resolve_first_launch_resource(app: &AppHandle, name: &str) -> Option<PathBuf> {
    if let Ok(p) = app
        .path()
        .resolve(format!("first_launch/{name}"), BaseDirectory::Resource)
    {
        if p.is_file() {
            return Some(p);
        }
    }
    let exe_dir = std::env::current_exe()
        .ok()
        .and_then(|p| p.parent().map(|d| d.to_path_buf()))?;
    let candidates = [
        exe_dir.join("first_launch").join(name),
        exe_dir.join("resources").join("first_launch").join(name),
        exe_dir
            .join("..")
            .join("Resources")
            .join("first_launch")
            .join(name),
        exe_dir
            .join("..")
            .join("resources")
            .join("first_launch")
            .join(name),
    ];
    candidates.into_iter().find(|p| p.is_file())
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

/// Keeps the machine awake while an agent session is running. The underlying
/// assertion (macOS IOPM / Windows SetThreadExecutionState / Linux systemd-inhibit)
/// is released when the handle is dropped or the app exits.
struct KeepAwakeState(Mutex<Option<keepawake::KeepAwake>>);

#[tauri::command]
fn prevent_sleep(state: tauri::State<'_, KeepAwakeState>) -> Result<(), String> {
    let mut guard = state.0.lock().map_err(|e| e.to_string())?;
    if guard.is_none() {
        *guard = Some(
            keepawake::Builder::default()
                .idle(true)
                .reason("Agent session running")
                .app_name("Danmo Work")
                .app_reverse_domain("com.danmo.work")
                .create()
                .map_err(|e| e.to_string())?,
        );
    }
    Ok(())
}

#[tauri::command]
fn allow_sleep(state: tauri::State<'_, KeepAwakeState>) -> Result<(), String> {
    state.0.lock().map_err(|e| e.to_string())?.take();
    Ok(())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .manage(KeepAwakeState(Mutex::new(None)))
        .invoke_handler(tauri::generate_handler![
            open_external,
            write_file_bytes,
            prevent_sleep,
            allow_sleep
        ])
        .setup(|app| {
            handle_first_launch(&app.handle());
            if let Err(e) = spawn_backend(&app.handle()) {
                eprintln!("WARNING: backend sidecar failed to start: {e}");
                eprintln!("The app will run without backend API. Start it manually if needed.");
            }
            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error while building tauri application")
        .run(|app_handle, event| match event {
            // ExitRequested fires first (window close / Cmd+Q); Exit is last chance.
            // Without this, the Go sidecar keeps listening on :7801 forever.
            tauri::RunEvent::ExitRequested { .. } | tauri::RunEvent::Exit => {
                shutdown_sidecar(&app_handle);
            }
            _ => {}
        });
}
