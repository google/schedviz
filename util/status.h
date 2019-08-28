#ifndef SCHEDVIZ_UTIL_STATUS_H_
#define SCHEDVIZ_UTIL_STATUS_H_

#include <string>

#include "absl/base/attributes.h"
#include "absl/strings/string_view.h"

enum class StatusCode : int {
  kOk = 0,
  kInternal = 13,
};

class ABSL_MUST_USE_RESULT Status final {
  StatusCode code_;
  std::string message_;

  Status(StatusCode code, absl::string_view message)
      : code_(code), message_(message) {}

 public:
  Status() : code_(StatusCode::kOk), message_("") {}

  inline static Status InternalError(absl::string_view msg) {
    return Status(StatusCode::kInternal, msg);
  }

  inline static Status OkStatus() { return Status(); }

  StatusCode code() const { return code_; }

  ABSL_MUST_USE_RESULT bool ok() const { return code_ == StatusCode::kOk; }

  absl::string_view message() const { return message_; }
};

#endif  // SCHEDVIZ_UTIL_STATUS_H_
