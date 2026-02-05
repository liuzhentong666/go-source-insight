func TestNewComplexityAnalyzer(t *testing.T) {
	type args struct {
	}
	tests := []struct {
		name string
		args args
		want *ComplexityAnalyzer
	}{
		{
			name: "TODO: 测试用例描述",
			args: args{},
			want: TODO_ * ComplexityAnalyzer,
		},
		// TODO: 添加更多测试用例
		// {
		//     name: "边界值测试",
		//     args: args{ },
		//     want: TODO_*ComplexityAnalyzer,
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got != tt.want {
				t.Errorf("%s() = %v, want %v", got, tt.want)
			}
		})
	}
}


