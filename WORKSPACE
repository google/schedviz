workspace(
    name = "schedviz",
    managed_directories = {"@npm": ["node_modules"]},
)

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/rules_go/releases/download/0.18.6/rules_go-0.18.6.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/0.18.6/rules_go-0.18.6.tar.gz",
    ],
    sha256 = "f04d2373bcaf8aa09bccb08a98a57e721306c8f6043a2a0ee610fd6853dcde3d",
)

http_archive(
    name = "com_grail_bazel_toolchain",
    sha256 = "015454eb86330cd20bce951468652ce572e8c04421eda456926ea658d861a939",
    strip_prefix = "bazel-toolchain-8570c4ccb39f750452b0b5607c9f54a093214f26",
    urls = ["https://github.com/grailbio/bazel-toolchain/archive/8570c4ccb39f750452b0b5607c9f54a093214f26.zip"],
)

http_archive(
    name = "rules_cc",
    sha256 = "2cc55c30570dfbff6c4c205bc1f6cb2a799746117bc51337f399f49991d39f8b",
    strip_prefix = "rules_cc-12a2d801f69ca8fff9128a8044549d7cbac306f1",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_cc/archive/12a2d801f69ca8fff9128a8044549d7cbac306f1.zip",
        "https://github.com/bazelbuild/rules_cc/archive/12a2d801f69ca8fff9128a8044549d7cbac306f1.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    urls = ["https://github.com/bazelbuild/bazel-gazelle/releases/download/0.17.0/bazel-gazelle-0.17.0.tar.gz"],
    sha256 = "3c681998538231a2d24d0c07ed5a7658cb72bfb5fd4bf9911157c0e9ac6a2687",
)

http_archive(
    name = "build_bazel_rules_nodejs",
    sha256 = "6d4edbf28ff6720aedf5f97f9b9a7679401bf7fca9d14a0fff80f644a99992b4",
    urls = ["https://github.com/bazelbuild/rules_nodejs/releases/download/0.32.2/rules_nodejs-0.32.2.tar.gz"],
)

http_archive(
    name = "io_bazel_rules_sass",
    sha256 = "b4ddeab9835779d7f929786f9d0c9724e12501d28c6647e56b3af14f53617cb3",
    strip_prefix = "rules_sass-b69e8f5a6f0537e40eadc45a22367ac3c90d1cd4",
    url = "https://github.com/bazelbuild/rules_sass/archive/b69e8f5a6f0537e40eadc45a22367ac3c90d1cd4.zip",
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "87fc6a2b128147a0a3039a2fd0b53cc1f2ed5adb8716f50756544a572999ae9a",
    strip_prefix = "rules_docker-0.8.1",
    urls = ["https://github.com/bazelbuild/rules_docker/archive/v0.8.1.tar.gz"],
)

load("@build_bazel_rules_nodejs//:defs.bzl", "check_bazel_version", "node_repositories", "yarn_install")

check_bazel_version(minimum_bazel_version = "0.27.0")

load("@com_grail_bazel_toolchain//toolchain:rules.bzl", "llvm_toolchain")

llvm_toolchain(
    name = "llvm_toolchain",
    llvm_version = "9.0.0",
)

load("@llvm_toolchain//:toolchains.bzl", "llvm_register_toolchains")

llvm_register_toolchains()

# Setup the Node repositories. We need a NodeJS version that is more recent than v10.15.0
# because "selenium-webdriver" which is required for "ng e2e" cannot be installed.
# TODO: remove the custom repositories once "rules_nodejs" supports v10.16.0 by default.
node_repositories(
    node_repositories = {
        "10.16.0-darwin_amd64": ("node-v10.16.0-darwin-x64.tar.gz", "node-v10.16.0-darwin-x64", "6c009df1b724026d84ae9a838c5b382662e30f6c5563a0995532f2bece39fa9c"),
        "10.16.0-linux_amd64": ("node-v10.16.0-linux-x64.tar.xz", "node-v10.16.0-linux-x64", "1827f5b99084740234de0c506f4dd2202a696ed60f76059696747c34339b9d48"),
        "10.16.0-windows_amd64": ("node-v10.16.0-win-x64.zip", "node-v10.16.0-win-x64", "aa22cb357f0fb54ccbc06b19b60e37eefea5d7dd9940912675d3ed988bf9a059"),
    },
    node_version = "10.16.0",
)

yarn_install(
    name = "npm",
    package_json = "//:package.json",
    yarn_lock = "//:yarn.lock",
)

load("@npm//:install_bazel_dependencies.bzl", "install_bazel_dependencies")

install_bazel_dependencies()

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()
go_register_toolchains()

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

go_repository(
    name = "com_github_gorilla_mux",
    importpath = "github.com/gorilla/mux",
    tag = "v1.7.0",
)
go_repository(
    name = "com_github_google_go-cmp",
    importpath = "github.com/google/go-cmp",
    tag = "v0.3.0",
)
go_repository(
    name = "com_github_google_uuid",
    importpath = "github.com/google/uuid",
    tag = "v1.1.1",
)
go_repository(
    name = "com_github_golang_protobuf",
    importpath = "github.com/golang/protobuf",
    tag = "v1.3.1",
)
go_repository(
    name = "org_golang_google_grpc",
    importpath = "google.golang.org/grpc",
    tag = "v1.20.1",
)
go_repository(
    name = "com_github_workiva_go-datastructures",
    importpath = "github.com/Workiva/go-datastructures",
    tag = "v1.0.50",
)
go_repository(
    name = "com_github_hashicorp_golang-lru",
    importpath = "github.com/hashicorp/golang-lru",
    tag = "v0.5.1",
)
go_repository(
    name = "com_github_golang_glog",
    importpath = "github.com/golang/glog",
    tag = "23def4e6c14b4da8ac2ed8007337bc5eb5007998",
)
go_repository(
    name = "com_github_golang_time",
    importpath = "golang.org/x/time",
    tag = "c4c64cad1fd0a1a8dab2523e04e61d35308e131e",
)
go_repository(
    name = "org_golang_x_sync",
    importpath = "github.com/golang/sync",
    tag = "112230192c580c3556b8cee6403af37a4fc5f28c",
)
go_repository(
    name = "com_github_ilhamster_ltl",
    importpath = "github.com/ilhamster/ltl",
    tag = "a7018a511f62f2326be6234934efbbeb44826475",
)

# abseil-cpp
http_archive(
    name = "com_google_absl",
    urls = ["https://github.com/abseil/abseil-cpp/archive/20190808.zip"],
    strip_prefix = "abseil-cpp-20190808",
    sha256 = "0b62fc2d00c2b2bc3761a892a17ac3b8af3578bd28535d90b4c914b0a7460d4e",
)

# re2
http_archive(
    name = "com_googlesource_code_re2",
    urls = ["https://github.com/google/re2/archive/2019-08-01.zip"],
    strip_prefix = "re2-2019-08-01",
    sha256 = "ae686c2f48e8df31414476a5e8dea4221c6fa679c0444470ab8703c1730e51dc",
)

load("@npm_bazel_typescript//:defs.bzl", "ts_setup_workspace")

ts_setup_workspace()

load("@npm_bazel_karma//:package.bzl", "rules_karma_dependencies")

rules_karma_dependencies()

load("@npm_bazel_karma//:browser_repositories.bzl", "browser_repositories")
load("@io_bazel_rules_webtesting//web:repositories.bzl", "web_test_repositories")

web_test_repositories()
browser_repositories()

load("@io_bazel_rules_sass//:defs.bzl", "sass_repositories")

sass_repositories()

load("@io_bazel_rules_docker//repositories:repositories.bzl", container_repositories = "repositories")
load("@io_bazel_rules_docker//toolchains/docker:toolchain.bzl", docker_toolchain_configure = "toolchain_configure")

# Update this path if Docker is installed in a different location (for example, when using Windows).
docker_toolchain_configure(
    name = "docker_config",
    docker_path = "/usr/bin/docker",
)

container_repositories()

load("@io_bazel_rules_docker//go:image.bzl", _go_image_repos = "repositories")

_go_image_repos()
