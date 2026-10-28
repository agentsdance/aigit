class Aigit < Formula
  desc "AI-powered Git commit message generator using LLM"
  homepage "https://github.com/agentsdance/aigit"
  url "https://github.com/agentsdance/aigit/archive/refs/tags/v0.2.0.tar.gz"
  sha256 "7ec72d50a5140c6bdd6cb5827e9dd6a63a7b7a294f2e2c4d653bef3a71dba476"
  license "Apache-2.0"
  head "https://github.com/agentsdance/aigit.git", branch: "master"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X main.Version=#{version}
    ]

    system "go", "build", *std_go_args(ldflags: ldflags), "."
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/aigit version")
    assert_match "Generate git commit message", shell_output("#{bin}/aigit help")
  end

  def caveats
    <<~EOS
      Before using aigit, configure an AI provider:
        aigit auth add openai YOUR_API_KEY
        aigit auth add gemini YOUR_API_KEY
        aigit auth add deepseek YOUR_API_KEY
        aigit auth add doubao YOUR_API_KEY YOUR_ENDPOINT_ID

      Then use: aigit commit
    EOS
  end
end
