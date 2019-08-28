#include "util/trace.h"

#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/sysinfo.h>
#include <unistd.h>

#include <filesystem>
#include <fstream>
#include <iostream>
#include <string>
#include <utility>
#include <vector>

#include "absl/flags/flag.h"
#include "absl/flags/parse.h"
#include "absl/strings/str_cat.h"
#include "absl/strings/str_split.h"
#include "absl/strings/string_view.h"
#include "absl/time/clock.h"
#include "absl/time/time.h"
#include "re2/re2.h"
#include "util/status.h"

// Command line flags
ABSL_FLAG(std::string, out, "", "Path to directory to save trace in");
ABSL_FLAG(int, capture_seconds, 0, "Number of seconds to record a trace");
ABSL_FLAG(int, buffer_size, 4096,
          "Size of the trace buffer in KB. Default 4096");
ABSL_FLAG(std::vector<std::string>, events,
          std::vector<std::string>({
              "sched:sched_switch",
              "sched:sched_wakeup",
              "sched:sched_wakeup_new",
              "sched:sched_migrate_task",
          }),
          "Comma separated list of FTrace events to collect. Defaults to the "
          "scheduling events.");
ABSL_FLAG(std::string, kernel_trace_root, "/sys/kernel/debug/tracing",
          "Path to the root directory of the Ftrace filesystem");
ABSL_FLAG(std::string, kernel_devices_root, "/sys/devices",
          "Path to the root directory of the devices filesystem");

static constexpr const auto kUSAGE =
    "Usage: trace --out OUT --capture_seconds CAPTURE_SECONDS [OPTIONS]\n"
    "This program collects an FTrace trace for a specified period of time"
    "and saves the results to a tar.gz file\n"
    "\n"
    "OUT is the path to directory to save trace in\n"
    "CAPTURE_SECONDS is the number of seconds to record a trace for\n"
    "\n"
    "OPTIONS are"
    "\n"
    "--buffer_size Size of the trace buffer in KB. Default 4096\n"
    "--events Comma separated list of FTrace events to collect. Defaults to "
    "the scheduling events."
    "--kernel_trace_root Path to the root directory of the Ftrace filesystem. "
    "Default '/sys/kernel/debug/tracing'\n"
    "--kernel_devices_root Path to the root directory of the devices "
    "filesystem. Default '/sys/devices'";

/**
 * Regex for matching a CPU name in a SysFS path.
 */
static constexpr const LazyRE2 kCPURegex = {"(cpu\\d+$)"};

/**
 * Regex for matching a NUMA node name in a SysFS path.
 */
static constexpr const LazyRE2 kNodeRegex = {"(node\\d+$)"};

int main(int argc, char** argv) {
  absl::ParseCommandLine(argc, argv);

  if (geteuid() != 0) {
    std::cerr
        << "The trace collector must be run as root in order to access FTrace"
        << std::endl;
    return 1;
  }

  const auto& kernel_trace_root =
      std::filesystem::path(absl::GetFlag(FLAGS_kernel_trace_root));
  const auto& kernel_devices_root =
      std::filesystem::path(absl::GetFlag(FLAGS_kernel_devices_root));
  const auto& capture_seconds = absl::GetFlag(FLAGS_capture_seconds);
  const auto& buffer_size = absl::GetFlag(FLAGS_buffer_size);
  const auto& events = absl::GetFlag(FLAGS_events);
  const auto& output_path = std::filesystem::path(absl::GetFlag(FLAGS_out));

  if (output_path.string().empty()) {
    std::cerr << kUSAGE << std::endl;
    std::cerr << "--out is required." << std::endl;
    return 1;
  }
  if (capture_seconds <= 0) {
    std::cerr << "--capture_seconds must be greater than zero" << std::endl;
    return 1;
  }
  if (buffer_size <= 0) {
    std::cerr << "--buffer_size must be greater than zero" << std::endl;
    return 1;
  }
  if (!std::filesystem::exists(kernel_trace_root)) {
    std::cerr << "Path provided to --kernel_trace_root, " << kernel_trace_root
              << " does not exist" << std::endl;
    return 1;
  }
  if (!std::filesystem::exists(kernel_devices_root)) {
    std::cerr << "Path provided to --kernel_devices_root, " << kernel_trace_root
              << " does not exist" << std::endl;
    return 1;
  }

  FTraceTracer tracer(kernel_trace_root, kernel_devices_root, output_path,
                      buffer_size, events);

  const auto& status = tracer.Trace(capture_seconds);
  if (!status.ok()) {
    std::cerr << status.message() << std::endl;
    return 1;
  }

  return 0;
}

FTraceTracer::~FTraceTracer() {
  // Ignore error as we can't recover here.
  (void)StopTrace(/*final_copy=*/false);
}

Status FTraceTracer::Trace(int capture_seconds) {
  if (is_tracing_) {
    return Status::InternalError("Already Tracing");
  }

  std::cout << "Trace date "
            << absl::FormatTime("%Y-%m-%d %H:%M:%S", absl::Now(),
                                absl::LocalTimeZone())
            << ": capture for " << capture_seconds
            << " seconds, send output to " << output_path_ << std::endl;

  char temp_name[L_tmpnam];
  if (!std::tmpnam(temp_name)) {
    return Status::InternalError("Unable to create temporary directory.");
  }
  temp_path_ = std::filesystem::temp_directory_path() / temp_name;

  Status status;
  status = ConfigureFTrace();
  if (!status.ok()) {
    return status;
  }

  status = CopyFormats();
  if (!status.ok()) {
    return status;
  }

  status = CopySystemTopology();
  if (!status.ok()) {
    return status;
  }

  status = CollectTrace(capture_seconds);
  if (!status.ok()) {
    return status;
  }

  status = CreateTar("trace.tar.gz");
  if (!status.ok()) {
    return status;
  }

  std::cout << "Trace capture finished at "
            << absl::FormatTime("%Y-%m-%d %H:%M:%S", absl::Now(),
                                absl::LocalTimeZone())
            << std::endl;

  return Status::OkStatus();
}

Status FTraceTracer::ConfigureFTrace() {
  if (is_tracing_) {
    return Status::InternalError("Already Tracing");
  }
  Status status;
  // Disable tracing.
  status = WriteString(kernel_trace_root_ / "tracing_on", "0");
  if (!status.ok()) {
    return status;
  }

  // Hold a reference to the free_buffer file.
  // If this fd is closed, the buffer will be cleared.
  free_fd_ = open((kernel_trace_root_ / "free_buffer").c_str(), O_RDONLY);
  if (free_fd_ < 0) {
    return Status::InternalError("unable to open free_buffer file");
  }

  // Remove all current tracers from tracing.
  status = WriteString(kernel_trace_root_ / "current_tracer", "nop");
  if (!status.ok()) {
    return status;
  }

  // Stop tracing if this process ends or if we close the free_buffer file.
  status = WriteString(kernel_trace_root_ / "trace_options", "disable_on_free");
  if (!status.ok()) {
    return status;
  }

  // Set buffer size.
  status = WriteString(kernel_trace_root_ / "buffer_size_kb",
                       std::to_string(buffer_size_));
  if (!status.ok()) {
    return status;
  }

  // Enable events to record.
  status = EnableEvents();
  if (!status.ok()) {
    return status;
  }

  return Status::OkStatus();
}

Status FTraceTracer::EnableEvents() {
  if (is_tracing_) {
    return Status::InternalError("Already Tracing");
  }
  const auto& events_path = (kernel_trace_root_ / "set_event");
  int fd = open(events_path.c_str(), O_WRONLY | O_TRUNC, 0666);
  if (fd < 0) {
    return Status::InternalError(
        absl::StrCat("Could not open ", events_path.string()));
  }
  for (const auto& event : events_) {
    if ((unsigned)write(fd, event.c_str(), event.size()) != event.size()) {
      close(fd);
      return Status::InternalError(absl::StrCat("Failed to write ", event,
                                                " to ", events_path.string()));
    }
  }
  close(fd);

  return Status::OkStatus();
}

Status FTraceTracer::CopyFormats() {
  if (is_tracing_) {
    return Status::InternalError("Already Tracing");
  }
  const auto& out = temp_path_ / "formats";
  const std::filesystem::path& formats_root = kernel_trace_root_ / "events";
  for (const auto& event_type : events_) {
    std::filesystem::path event_format_path;
    const auto& event_type_parts = absl::StrSplit(event_type, ':');
    for (const auto& part : event_type_parts) {
      event_format_path /= std::string(part);
    }

    const auto& out_path = out / event_format_path;
    if (!std::filesystem::create_directories(out_path)) {
      return Status::InternalError(absl::StrCat(
          "Unable to create directories for path: ", out_path.string()));
    }
    const auto& status = CopyFakeFile(
        formats_root / event_format_path / "format", out_path / "format");
    if (!status.ok()) {
      return status;
    }
  }

  const auto& status =
      CopyFakeFile(formats_root / "header_page", out / "header_page");
  if (!status.ok()) {
    return status;
  }

  return Status::OkStatus();
}

Status FTraceTracer::CopySystemTopology() {
  if (is_tracing_) {
    return Status::InternalError("Already Tracing");
  }
  const auto& out = temp_path_ / "topology";
  const auto& node_root = kernel_devices_root_ / "system" / "node";
  for (const auto& node_entry :
       std::filesystem::directory_iterator(node_root)) {
    const auto& node_file_name = node_entry.path().filename().string();
    std::string node_name;

    if (!RE2::FullMatch(node_file_name, *kNodeRegex, &node_name)) {
      continue;
    }

    for (const auto& cpu_entry :
         std::filesystem::directory_iterator(node_entry)) {
      const auto& cpu_file_name = cpu_entry.path().filename().string();
      std::string cpu_name;

      if (!RE2::FullMatch(cpu_file_name, *kCPURegex, &cpu_name)) {
        continue;
      }

      const auto& topology_path = cpu_entry.path() / "topology";
      if (std::filesystem::exists(topology_path) &&
          std::filesystem::is_directory(topology_path)) {
        const auto& out_path = out / node_name / cpu_name / "topology";
        if (!std::filesystem::create_directories(out_path)) {
          return Status::InternalError(absl::StrCat(
              "Unable to create directories for path: ", out_path.string()));
        }
        for (const auto& topology_file_entry :
             std::filesystem::directory_iterator(topology_path)) {
          const auto& topology_filename = topology_file_entry.path().filename();
          const auto& status =
              CopyFakeFile(topology_file_entry, out_path / topology_filename);
          if (!status.ok()) {
            return status;
          }
        }
      }
    }
  }

  return Status::OkStatus();
}

Status FTraceTracer::CollectTrace(const int capture_seconds) {
  if (is_tracing_) {
    return Status::InternalError("Already Tracing");
  }
  // Prepare
  const auto& out = temp_path_ / "traces";
  // Create directories if they don't exist.
  if (!std::filesystem::exists(out)) {
    if (!std::filesystem::create_directories(out)) {
      return Status::InternalError(absl::StrCat(
          "Unable to create directories for path: ", out.string()));
    }
  }

  const auto& cpu_count = get_nprocs();
  ClearCPUFDs();
  fds_.reserve(cpu_count);
  for (int i = 0; i < cpu_count; i++) {
    const auto& cpuName = "cpu" + std::to_string(i);
    const auto& cpuPath =
        kernel_trace_root_ / "per_cpu" / cpuName / "trace_pipe_raw";
    const auto& outPath = out / cpuName;

    int in_fd = open(cpuPath.c_str(), O_RDONLY | O_NONBLOCK);
    if (in_fd == -1) {
      return Status::InternalError(
          absl::StrCat("Unable to open ", cpuPath.string()));
    }
    int out_fd =
        open(outPath.c_str(), O_CREAT | O_WRONLY | O_TRUNC | O_LARGEFILE, 0644);
    if (out_fd == -1) {
      return Status::InternalError(
          absl::StrCat("Unable to create ", outPath.string()));
    }
    fds_.emplace_back(std::make_pair(in_fd, out_fd));
  }

  // Start Trace.
  Status status;
  status = WriteString(kernel_trace_root_ / "tracing_on", "1");
  if (!status.ok()) {
    return status;
  }
  is_tracing_ = true;

  std::cout << "Waiting " << capture_seconds << " seconds" << std::endl;

  // Wait for trace to end.
  const auto& start_time = absl::Now();
  const auto& interval = absl::Milliseconds(100);
  absl::SleepFor(interval);
  Status failedCopyStatus;
  while (absl::Now() <= start_time + absl::Seconds(capture_seconds)) {
    status = CopyCPUBuffers();
    if (!status.ok()) {
      failedCopyStatus = status;
      break;
    }
    absl::SleepFor(interval);
  }

  status = StopTrace(/*final_copy=*/true);
  if (!failedCopyStatus.ok()) {
    // Merge the failure message from the failed copy and StopTrace,
    // if it failed.
    return Status::InternalError(
        absl::StrCat(failedCopyStatus.message(), "\n\n", status.message()));
  }
  return status;
}

Status FTraceTracer::CopyCPUBuffers() {
  if (!is_tracing_) {
    return Status::InternalError("Not currently in a trace");
  }
  const auto& cpu_count = get_nprocs();
  for (int i = 0; i < cpu_count; i++) {
    const auto& cpu_fds = fds_[i];
    const auto& status = CopyCPUBuffer(cpu_fds.first, cpu_fds.second);
    if (!status.ok()) {
      return status;
    }
  }

  return Status::OkStatus();
}

Status FTraceTracer::StopTrace(bool final_copy) {
  if (!is_tracing_) {
    return Status::InternalError("Not currently in a trace");
  }
  const auto& tracing_file_path = kernel_trace_root_ / "tracing_on";
  Status status;
  status = WriteString(tracing_file_path, "0");
  if (!status.ok()) {
    std::cerr << "WARNING: Failed to stop tracing. FTrace may still be "
                 "running. Double check that "
              << tracing_file_path << " is set to '0'" << std::endl;
    close(free_fd_);
    is_tracing_ = false;
    return status;
  }

  if (final_copy) {
    status = CopyCPUBuffers();
    if (!status.ok()) {
      close(free_fd_);
      is_tracing_ = false;
      return status;
    }
  }

  ClearCPUFDs();

  close(free_fd_);
  is_tracing_ = false;
  return status;
}

Status FTraceTracer::CopyCPUBuffer(int in_fd, int out_fd) {
  if (!is_tracing_) {
    return Status::InternalError("Not currently in a trace");
  }
  std::vector<char> trace_data(buffer_size_);

  while (true) {
    const auto bytes_read = read(in_fd, &trace_data.front(), trace_data.size());
    if (bytes_read == -1 && errno != EAGAIN) {
      return Status::InternalError(
          absl::StrCat("Unable to read cpu file ", std::to_string(in_fd)));
    }
    if (bytes_read == -1 || bytes_read == 0) {
      break;
    }

    write(out_fd, &trace_data.front(), bytes_read);
  }
  return Status::OkStatus();
}

Status FTraceTracer::CreateTar(const std::string& tar_name) {
  if (is_tracing_) {
    return Status::InternalError("Trace should be done before creating a tar");
  }
  const auto& out = output_path_ / tar_name;
  const auto& tempStr = temp_path_.string();
  const auto& outStr = out.string();
  const auto& tar_cmd =
      absl::StrCat("chmod -R a+rwX ", tempStr, " && ", "cd ", tempStr,
                   " && "
                   "tar -zcf ",
                   outStr, " *", " && ", "chmod a+rw ", outStr);
  if (system(tar_cmd.c_str()) != 0) {
    return Status::InternalError("Error running tar");
  }
  return Status::OkStatus();
}

void FTraceTracer::ClearCPUFDs() {
  for (const auto& cpu_fds : fds_) {
    close(cpu_fds.first);
    close(cpu_fds.second);
  }
  fds_.clear();
}

Status FTraceTracer::CopyFakeFile(const std::filesystem::path& src,
                                  const std::filesystem::path& dst) {
  std::ifstream in(src);
  std::ofstream out(dst);
  out << in.rdbuf();
  out.close();
  in.close();
  if (in.bad() || out.bad()) {
    return Status::InternalError(absl::StrCat("Failed to copy ", src.string()));
  }

  return Status::OkStatus();
}

Status FTraceTracer::WriteString(const std::filesystem::path& path,
                                 const std::string& data) {
  std::ofstream out(path);
  out << data;
  out.close();
  if (out.bad()) {
    return Status::InternalError(
        absl::StrCat("Failed to write to", path.string()));
  }
  return Status::OkStatus();
}
