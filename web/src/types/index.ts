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
  status: 'pending' | 'running' | 'completed' | 'failed';
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
