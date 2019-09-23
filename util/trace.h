#ifndef SCHEDVIZ_UTIL_TRACE_H_
#define SCHEDVIZ_UTIL_TRACE_H_

#include <unistd.h>

#include <filesystem>
#include <iostream>
#include <string>
#include <vector>

#include "util/status.h"

class FTraceTracer {
 public:
  /**
   * Constructs a new FTraceTracer.
   * @param kernel_trace_root Path to the root directory of the Ftrace
   * filesystem.
   * @param kernel_devices_root Path to the root directory of the devices
   * filesystem.
   * @param output_path Path to directory to save trace in.
   * @param buffer_size The number of kilobytes each CPU buffer will hold.
   * @param events A list of FTrace event names to record.
   */
  FTraceTracer(std::filesystem::path kernel_trace_root,
               std::filesystem::path kernel_devices_root,
               std::filesystem::path output_path, int buffer_size,
               std::vector<std::string> events)
      : kernel_trace_root_(std::move(kernel_trace_root)),
        kernel_devices_root_(std::move(kernel_devices_root)),
        output_path_(std::move(output_path)),
        buffer_size_(buffer_size),
        events_(std::move(events)) {}

  ~FTraceTracer();

  /**
   * Captures a new trace.
   * @param capture_seconds How long to capture a trace for.
   * @return Status if successful or not.
   */
  Status Trace(int capture_seconds);

 private:
  /**
   * Prepare FTrace for a new trace.
   * @return Status if successful or not.
   */
  Status ConfigureFTrace();

  /**
   * Enable tracing of the events provided to the constructor.
   * @return Status if successful or not.
   */
  Status EnableEvents();

  /**
   * Stop tracing and drain what's left of the per cpu buffers.
   * @param final_copy Whether or not to perform a final copy of the
   *                   trace buffer.
   * @return Status if successful or not.
   */
  Status StopTrace(bool final_copy);

  /**
   * Copies the format files for the provided events to the temp directory.
   * @return Status if successful or not.
   */
  Status CopyFormats();

  /**
   * Copies the system topology files for this machine to the temp directory.
   * @return Status if successful or not.
   */
  Status CopySystemTopology();

  /**
   * Collects a trace and writes it to the temp directory.
   * @param capture_seconds How long to collect the trace for.
   * @return Status if successful or not.
   */
  Status CollectTrace(int capture_seconds);

  /**
   * Copies all CPU buffers to the temp directory.
   * @return Status if successful or not.
   */
  Status CopyCPUBuffers();

  /**
   * Copies a CPU buffer from FTrace to out_fd.
   * @param in_fd File Descriptor for the FTrace cpu buffer pipe.
   * @param out_fd File descriptor to write to.
   * @return Status if successful or not.
   */
  Status CopyCPUBuffer(int in_fd, int out_fd);

  /**
   * Tars and gzips the directory located at "temp_path_" and writes it to
   * "output_path_".
   * @param tar_name The name of the tar file.
   * @return Status if successful or not.
   */
  Status CreateTar(const std::string& tar_name);

  /**
   * Clear and close the list of CPU file descriptors.
   */
  void ClearCPUFDs();

  /**
   * Copy a file from src to dst.
   * Uses I/O streams to handle reading fake files like those in FTrace that are
   * generated on demand.
   * @param src Path to the file to copy.
   * @param dst Path where the file copy should be written to.
   * @return Status if successful or not.
   */
  static Status CopyFakeFile(const std::filesystem::path& src,
                             const std::filesystem::path& dst);

  /**
   * Write a string to a file.
   * @param path Path to the file to write the string to.
   * @param data The string to write.
   * @return Status if successful or not.
   */
  static Status WriteString(const std::filesystem::path& path,
                            const std::string& data);

  // Path to the root directory of the Ftrace filesystem.
  const std::filesystem::path kernel_trace_root_;
  // Path to the root directory of the devices filesystem.
  const std::filesystem::path kernel_devices_root_;
  // Path to directory to save trace in.
  const std::filesystem::path output_path_;
  // Size of the trace buffer in KB.
  const int buffer_size_;
  // List of Ftrace Events.
  const std::vector<std::string> events_;

  // Path to temporary directory.
  std::filesystem::path temp_path_;

  // Are we currently running a trace or not?
  bool is_tracing_ = false;
  // File Descriptors for CPU buffers and output files. Indexed by CPU ID.
  std::vector<std::pair<int, int>> fds_;
  // File Descriptor for the free buffer file.
  // If closed, this will clear the kernel ring buffer.
  int free_fd_;
};


#endif  // SCHEDVIZ_UTIL_TRACE_H_
