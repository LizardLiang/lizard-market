#!/usr/bin/env node

/**
 * Simple Token Optimization Test
 * Compare file reading vs curated context approaches
 */

import { query } from "@anthropic-ai/claude-agent-sdk";
import { readFileSync, writeFileSync } from "fs";
import { join } from "path";

console.log("🧪 Simple Token Optimization Test");
console.log("=================================");

async function testTokenOptimization() {
  const testPrompts = {
    original: `
Task: Create a technical specification for a simple user authentication API.

Instructions:
1. Read plugins/kratos/templates/tech-spec-template.md for the format
2. Read plugins/kratos/references/agent-protocol.md for deliverable requirements
3. Read .claude/.Arena/project-overview.md if it exists for project context
4. Create a tech spec with: authentication endpoints, security considerations, data models

Create a detailed technical specification following the template format.
`,
    optimized: `
CURATED_CONTEXT:
{
  "role": "Technical Architect",
  "tech_spec_template": {
    "sections": ["# Architecture", "## Authentication Flow", "## API Endpoints", "## Security", "## Data Models"],
    "format": "Markdown with code blocks for examples"
  },
  "deliverable_requirements": {
    "must_include": ["Clear API definitions", "Security considerations", "Data validation rules"],
    "format": "Complete specification ready for implementation"
  },
  "project_context": {
    "stack": "Node.js + Express + JWT",
    "database": "PostgreSQL",
    "deployment": "Docker containers"
  }
}

Task: Create a technical specification for a simple user authentication API.

You have all necessary context above. Create a tech spec with: authentication endpoints, security considerations, data models.

Do NOT read additional files - everything you need is provided in CURATED_CONTEXT.
`
  };

  const results = {};

  for (const [approach, prompt] of Object.entries(testPrompts)) {
    console.log(`\n🔍 Testing ${approach} approach...`);

    const startTime = Date.now();
    let inputTokens = 0;
    let outputTokens = 0;
    let messages = 0;

    try {
      const stream = query({
        prompt,
        options: {
          model: "claude-sonnet-4-6",
          cwd: "C:\\Users\\lizard_liang\\personal\\ai-agents\\lizard-market\\plugins\\kratos"
        }
      });

      for await (const msg of stream) {
        messages++;

        // Extract token usage from messages
        if (msg.type === "assistant" && msg.message?.usage) {
          inputTokens += msg.message.usage.input_tokens || 0;
          outputTokens += msg.message.usage.output_tokens || 0;
        }

        // Show progress
        if (msg.type === "assistant" && msg.message?.content) {
          process.stdout.write(".");
        }
      }

      console.log(" ✅");

      results[approach] = {
        inputTokens,
        outputTokens,
        totalTokens: inputTokens + outputTokens,
        duration: Date.now() - startTime,
        messages,
        success: true
      };

      console.log(`   Input tokens: ${inputTokens}`);
      console.log(`   Output tokens: ${outputTokens}`);
      console.log(`   Total tokens: ${inputTokens + outputTokens}`);
      console.log(`   Duration: ${Math.round((Date.now() - startTime) / 1000)}s`);
      console.log(`   Messages: ${messages}`);

    } catch (error) {
      console.log(" ❌");
      console.log(`   Error: ${error.message}`);

      results[approach] = {
        inputTokens: 0,
        outputTokens: 0,
        totalTokens: 0,
        duration: Date.now() - startTime,
        messages,
        success: false,
        error: error.message
      };
    }
  }

  // Calculate improvements
  if (results.original.success && results.optimized.success) {
    const inputSavings = results.original.inputTokens - results.optimized.inputTokens;
    const inputSavingsPercent = (inputSavings / results.original.inputTokens * 100).toFixed(1);

    const totalSavings = results.original.totalTokens - results.optimized.totalTokens;
    const totalSavingsPercent = (totalSavings / results.original.totalTokens * 100).toFixed(1);

    console.log(`\n📊 RESULTS COMPARISON`);
    console.log(`=====================`);
    console.log(`Input token savings: ${inputSavings} tokens (${inputSavingsPercent}%)`);
    console.log(`Total token savings: ${totalSavings} tokens (${totalSavingsPercent}%)`);

    if (parseFloat(inputSavingsPercent) > 0) {
      console.log(`\n✅ OPTIMIZATION SUCCESSFUL!`);
      console.log(`Context curation reduced input tokens by ${inputSavingsPercent}%`);
    } else {
      console.log(`\n❌ OPTIMIZATION FAILED`);
      console.log(`No significant token reduction achieved`);
    }

    // Save results
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
    writeFileSync(
      `token-test-${timestamp}.json`,
      JSON.stringify({
        timestamp,
        results,
        savings: {
          inputTokens: inputSavings,
          inputPercent: inputSavingsPercent,
          totalTokens: totalSavings,
          totalPercent: totalSavingsPercent
        }
      }, null, 2)
    );

    console.log(`\nResults saved to: token-test-${timestamp}.json`);

  } else {
    console.log(`\n❌ TEST FAILED`);
    console.log(`One or both approaches failed to complete`);
  }
}

// Run the test
testTokenOptimization().catch(console.error);