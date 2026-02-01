import re
import subprocess
from dataclasses import dataclass, field
from typing import List, Optional


@dataclass
class GoFunction:
    """Go 函数/方法定义"""
    name: str
    signature: str  # 完整签名
    description: str = ""
    receiver: str = ""  # 如果是方法，receiver 部分
    params: str = ""
    returns: str = ""
    package: str = ""


@dataclass
class GoType:
    """Go 类型定义（struct/interface）"""
    name: str
    kind: str  # "struct" or "interface"
    description: str = ""
    fields: List[str] = field(default_factory=list)
    methods: List[str] = field(default_factory=list)
    package: str = ""


@dataclass
class GoPackage:
    """解析后的 Go 包"""
    name: str
    import_path: str
    functions: List[GoFunction] = field(default_factory=list)
    types: List[GoType] = field(default_factory=list)
    description: str = ""


class GoDocParser:
    """使用 go doc 解析 Go SDK 接口"""

    def parse_package(self, package_path: str) -> GoPackage:
        """
        解析单个 Go 包
        Args:
            package_path: 包路径（目录或 import path）
        """
        # 获取包信息
        pkg_info = self._get_package_info(package_path)

        # 获取完整文档
        doc_output = self._run_go_doc(package_path)

        # 解析文档
        functions, types = self._parse_doc_output(doc_output)

        return GoPackage(
            name=pkg_info["name"],
            import_path=pkg_info["import_path"],
            functions=functions,
            types=types,
            description=pkg_info.get("doc", ""),
        )

    def parse_repository(self, repo_path: str) -> List[GoPackage]:
        """
        解析整个 Go 代码库（包含多个包）
        Args:
            repo_path: 代码库根目录
        """
        packages = []

        # 列出所有包
        result = subprocess.run(
            ["go", "list", "-json", "./..."],
            cwd=repo_path,
            capture_output=True,
            text=True,
        )

        if result.returncode != 0:
            raise RuntimeError(f"go list failed: {result.stderr}")

        for line in result.stdout.strip().split("\n"):
            if not line.strip():
                continue
            try:
                pkg_info = eval(line)
                pkg = self.parse_package(pkg_info["ImportPath"])
                packages.append(pkg)
            except (SyntaxError, KeyError):
                continue

        return packages

    def _get_package_info(self, package_path: str) -> dict:
        """获取包的基本信息"""
        result = subprocess.run(
            ["go", "list", "-json", package_path],
            capture_output=True,
            text=True,
        )

        if result.returncode != 0:
            # 尝试作为目录
            result = subprocess.run(
                ["go", "list", "-json", "./"],
                cwd=package_path,
                capture_output=True,
                text=True,
            )

        if result.returncode != 0:
            raise RuntimeError(f"Failed to get package info: {result.stderr}")

        return eval(result.stdout.strip())

    def _run_go_doc(self, package_path: str) -> str:
        """运行 go doc -all 获取完整文档"""
        result = subprocess.run(
            ["go", "doc", "-all", package_path],
            capture_output=True,
            text=True,
        )

        if result.returncode != 0:
            # 如果失败，尝试只解析当前目录
            result = subprocess.run(
                ["go", "doc", "-all", "./"],
                cwd=package_path,
                capture_output=True,
                text=True,
            )

        return result.stdout

    def _parse_doc_output(self, output: str) -> tuple:
        """解析 go doc 输出"""
        functions = []
        types = []

        current_package = ""
        lines = output.split("\n")

        i = 0
        while i < len(lines):
            line = lines[i].rstrip()

            # 包声明
            if line.startswith("package "):
                current_package = line.replace("package ", "").split(" ")[0]
                i += 1
                continue

            # 跳过空行和文件分隔符
            if not line or line.startswith(" " * 10) or line.startswith("CONSTANTS") or line.startswith("VARIABLES"):
                i += 1
                continue

            # 解析函数/方法
            func_match = self._parse_function(line)
            if func_match:
                functions.append(func_match)
                i += 1
                continue

            # 解析类型定义
            type_match = self._parse_type(line, lines, i)
            if type_match:
                types.append(type_match[0])
                i = type_match[1]
                continue

            i += 1

        return functions, types

    def _parse_function(self, line: str) -> Optional[GoFunction]:
        """解析函数/方法定义行"""
        # func FunctionName(params) returns
        # func (receiver) MethodName(params) returns

        pattern = r'^func(?:\s+\(([^)]+)\))?\s+([A-Z]\w+)\s*(\([^)]*\))?\s*(.*)$'
        match = re.match(pattern, line)

        if not match:
            return None

        receiver = match.group(1) or ""
        name = match.group(2)
        params = match.group(3) or "()"
        returns = match.group(4) or ""

        # 查找描述（可能在当前行或下一行）
        description = ""
        if "    " in line:
            description = line.split("    ")[-1].strip()

        # 清理 signature
        signature = f"func{(' (' + receiver + ')') if receiver else ''} {name}{params}"
        if returns:
            signature += f" {returns}"

        return GoFunction(
            name=name,
            signature=signature,
            description=description,
            receiver=receiver,
            params=params,
            returns=returns,
        )

    def _parse_type(self, line: str, lines: List[str], idx: int) -> tuple:
        """解析类型定义（struct/interface）"""
        # type TypeName struct {
        # type TypeName interface {

        pattern = r'^type\s+([A-Z]\w+)\s+(struct|interface)\s*\{?'
        match = re.match(pattern, line)

        if not match:
            return None, idx

        type_name = match.group(1)
        kind = match.group(2)

        typ = GoType(
            name=type_name,
            kind=kind,
        )

        # 收集字段/方法
        i = idx + 1
        while i < len(lines):
            content = lines[i]

            # 结束条件
            if not content.startswith(" ") or content.startswith("\t") is False:
                break

            content = content.strip()

            # 跳过空行
            if not content or content.startswith("//"):
                i += 1
                continue

            # 解析字段: FieldName Type
            field_match = re.match(r'^(\w+)\s+([^\s]+)(?:\s+(.*))?$', content)
            if field_match:
                field_str = content
                if field_match.group(3):
                    field_str = f"{field_match.group(1)} {field_match.group(2)} // {field_match.group(3)}"
                typ.fields.append(field_str)
            # 解析方法: MethodName(params) returns
            elif content.startswith("func"):
                typ.methods.append(content)

            i += 1

        return typ, i

    def format_for_embedding(self, package: GoPackage) -> List[str]:
        """格式化为嵌入向量"""
        chunks = []

        # 每个函数一个块
        for func in package.functions:
            chunk = f"""
Package: {package.name}
Function: {func.name}
Signature: {func.signature}
Description: {func.description}
""".strip()
            chunks.append(chunk)

        # 每个类型一个块
        for typ in package.types:
            fields_str = "\n".join(typ.fields) if typ.fields else "  (no exported fields)"
            methods_str = "\n".join(typ.methods) if typ.methods else ""

            chunk = f"""
Package: {package.name}
Type: {typ.name}
Kind: {typ.kind}
Fields:
{fields_str}
""".strip()

            if methods_str:
                chunk += f"\nMethods:\n{methods_str}"

            chunks.append(chunk)

        return chunks


# 测试
if __name__ == "__main__":
    import sys

    if len(sys.argv) < 2:
        print("Usage: python go.py <package_path>")
        sys.exit(1)

    parser = GoDocParser()
    packages = parser.parse_repository(sys.argv[1])

    for pkg in packages:
        print(f"\n=== Package: {pkg.name} ===")
        print(f"Import: {pkg.import_path}")
        print(f"\nFunctions:")
        for func in pkg.functions[:5]:
            print(f"  - {func.signature}")
            if func.description:
                print(f"    {func.description}")
        print(f"\nTypes:")
        for typ in pkg.types[:5]:
            print(f"  - {typ.name} ({typ.kind})")
