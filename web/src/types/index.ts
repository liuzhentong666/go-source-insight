// 用户相关类型
export interface User {
  id: number;
  username: string;
  email: string;
  created_at: string;
  updated_at: string;
}

export interface LoginRequest {
  username: string;
  password: string;
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
}

// 项目相关类型
export interface Project {
  id: number;
  name: string;
  description: string;
  owner_id: number;
  created_at: string;
  updated_at: string;
}

export interface CreateProjectRequest {
  name: string;
  description?: string;
}

// 分析相关类型
export interface Analysis {
  id: number;
  project_id: number;
  status: 'pending' | JobStatus;
  result: string;
  created_at: string;
  updated_at: string;
}

export interface AnalysisRequest {
  project_id: number;
  code: string;
}

// 分析结果类型
export interface FunctionResult {
  name: string;
  line: number;
  complexity: number;
  lines: number;
  issues: string[];
}

export interface ComplexityStatistics {
  total_functions: number;
  simple_functions: number;
  medium_functions: number;
  complex_functions: number;
  very_complex_functions: number;
}

export interface ComplexityResult {
  file: string;
  total: number;
  functions: FunctionResult[];
  summary: string;
  statistics: ComplexityStatistics;
}

export interface SecurityIssue {
  id: string;
  rule_id: string;
  severity: 'Critical' | 'High' | 'Medium' | 'Low';
  category: string;
  description: string;
  line: number;
  code_snippet: string;
  suggestion: string;
}

export interface SecurityStats {
  total_issues: number;
  critical: number;
  high: number;
  medium: number;
  low: number;
}

export interface SecurityResult {
  file: string;
  total: number;
  issues: SecurityIssue[];
  summary: string;
  statistics: SecurityStats;
}

export interface BugIssue {
  file?: string;
  confidence?: string;
  id: string;
  rule_id: string;
  severity: 'High' | 'Medium' | 'Low';
  category: string;
  description: string;
  line: number;
  code_snippet: string;
  fix_suggestion: string;
}

export interface BugStats {
  total_issues: number;
  high: number;
  medium: number;
  low: number;
}

export interface BugResult {
  language: string;
  status: string;
  total_files: number;
  analyzed_files: number;
  total: number;
  bugs: BugIssue[];
  summary: string;
  statistics: BugStats;
}

export interface AnalysisResult {
  complexity: ComplexityResult;
  security: SecurityResult;
  bugs: BugResult;
  analyzed_at: string;
}

export type JobStatus = 'queued' | 'running' | 'canceling' | 'canceled' | 'completed' | 'failed';
export type JobKind = 'analyze' | 'index' | 'legacy_analyze';
export interface JobAccepted { job_id: string; project_id: number; status: JobStatus }
export interface LegacyJobAccepted { job_id: string; analysis_id: number; status: JobStatus }
export interface Job extends JobAccepted {
  recovering?: boolean;
  scope_binding?: string;
  kind: JobKind;
  target: string;
  result: StaticReport | IndexSummary | AnalysisResult | null;
  failure_code?: string;
  failure_reason?: string;
  created_at: string;
  updated_at: string;
  started_at: string | null;
  finished_at: string | null;
  analysis_id?: number;
}
export interface ScanResult {
  target_path: string;
  total_files: number;
  production_file_count: number;
  test_file_count: number;
  total_lines: number;
  skipped_dirs: string[];
  files: string[];
  production_files: string[];
  test_files: string[];
}
export interface ProjectSourceInfo {
  project_id: number;
  source_mode?: 'local' | 'bound' | 'legacy';
  binding_configured: boolean;
  absolute_input_allowed: boolean;
  host_root?: string;
  binding_id: string;
  available: boolean;
  unavailable_code?: string;
  excluded_paths: string[];
}
export interface StaticFileReport {
  path: string;
  functions: number;
  complexity_issues: number;
  security_issues: number;
  bug_issues: number;
  complexity: ComplexityResult | null;
  security: SecurityResult | null;
  bugs: BugResult | null;
}
export interface StaticReport {
  target_path: string;
  files_analyzed: number;
  functions_analyzed: number;
  complexity_issues: number;
  security_issues: number;
  bug_issues: number;
  files: StaticFileReport[];
}
export interface IndexSummary {
  target_path: string;
  files_scanned: number;
  chunks_created: number;
  chunks_indexed: number;
  collection: string;
}
export interface SourceCitation {
  id: string;
  source: string;
  content: string;
  score: number;
  start_line: number;
  end_line: number;
  chunk_type: string;
}
export interface AskRequest { question: string; file?: string }
export interface AnswerResult { question: string; answer: string; sources: SourceCitation[] }
