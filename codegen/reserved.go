package codegen

import "strings"

// A name the source program chose is not always a name the target language
// will accept. examples/switch_stmt.xql.json calls its function `label`, which
// is a keyword in Pascal, and fpc refused the whole program at line 3 —
// "identifier expected but LABEL found" — while validate had reported the AST
// as fine. The AST was fine. Every backend was spelling identifiers straight
// through, so a name that is ordinary in one language and reserved in another
// produced a file that could not be parsed.
//
// The fix belongs here rather than in thirty-eight emitters. A collision is a
// property of the pair (program, target language), and one rewrite over the
// tree fixes it for every backend at once — the same trade lower_switch.go
// makes. Backends stay unaware: by the time one runs, no name in the tree
// collides with a keyword of the language it emits.
//
// The rewrite is functional, for the reason lower_switch.go documents: the
// conformance suite parses one file and compiles it to every target, so
// mutating the tree would leave the next target compiling the previous
// target's renames.

// words turns a space-separated list into a set, so the tables below read as
// keyword lists rather than as Go literals.
func words(list string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.Fields(list) {
		set[w] = true
	}
	return set
}

// reservedByLanguage holds, per language, the words that cannot be an
// identifier there. These are keywords the grammar reserves — not every name
// the standard library happens to use. Renaming more than necessary would
// churn output for no gain; renaming less leaves a program that will not
// parse, which is the defect this file exists for.
var reservedByLanguage = map[string]map[string]bool{
	"go": words(`break case chan const continue default defer else fallthrough
		for func go goto if import interface map package range return select
		struct switch type var`),

	"rust": words(`as async await break const continue crate dyn else enum
		extern false fn for if impl in let loop match mod move mut pub ref
		return self Self static struct super trait true type union unsafe use
		where while abstract become box do final macro override priv typeof
		unsized virtual yield try`),

	"js": words(`await break case catch class const continue debugger default
		delete do else enum export extends false finally for function if import
		in instanceof new null return super switch this throw true try typeof
		var void while with yield let static implements interface package
		private protected public`),

	"ts": words(`await break case catch class const continue debugger default
		delete do else enum export extends false finally for function if import
		in instanceof new null return super switch this throw true try typeof
		var void while with yield let static implements interface package
		private protected public any boolean number string symbol declare
		namespace readonly type`),

	"java": words(`abstract assert boolean break byte case catch char class
		const continue default do double else enum extends final finally float
		for goto if implements import instanceof int interface long native new
		package private protected public return short static strictfp super
		switch synchronized this throw throws transient try void volatile while
		true false null`),

	"csharp": words(`abstract as base bool break byte case catch char checked
		class const continue decimal default delegate do double else enum event
		explicit extern false finally fixed float for foreach goto if implicit
		in int interface internal is lock long namespace new null object
		operator out override params private protected public readonly ref
		return sbyte sealed short sizeof stackalloc static string struct switch
		this throw true try typeof uint ulong unchecked unsafe ushort using
		virtual void volatile while`),

	"kotlin": words(`as break class continue do else false for fun if in
		interface is null object package return super this throw true try
		typealias typeof val var when while`),

	"swift": words(`associatedtype class deinit enum extension fileprivate func
		import init inout internal let open operator private protocol public
		rethrows static struct subscript typealias var break case continue
		default defer do else fallthrough for guard if in repeat return switch
		where while as Any catch false is nil super self Self throw throws true
		try`),

	"py": words(`False None True and as assert async await break class continue
		def del elif else except finally for from global if import in is lambda
		nonlocal not or pass raise return try while with yield`),

	"dart": words(`assert break case catch class const continue default do else
		enum extends false final finally for if in is new null rethrow return
		super switch this throw true try var void while with abstract as
		covariant deferred dynamic export extension external factory Function
		get implements import interface late library mixin operator part
		required set static typedef async await yield`),

	"lua": words(`and break do else elseif end false for function goto if in
		local nil not or repeat return then true until while`),

	"ruby": words(`BEGIN END alias and begin break case class def defined do
		else elsif end ensure false for if in module next nil not or redo
		rescue retry return self super then true undef unless until when while
		yield`),

	"php": words(`abstract and array as break callable case catch class clone
		const continue declare default do echo else elseif empty enddeclare
		endfor endforeach endif endswitch endwhile enum extends final finally
		fn for foreach function global goto if implements include include_once
		instanceof insteadof interface isset list match namespace new or print
		private protected public readonly require require_once return static
		switch throw trait try unset use var while xor yield`),

	"zig": words(`addrspace align allowzero and anyframe anytype asm async
		await break callconv catch comptime const continue defer else enum
		errdefer error export extern fn for if inline linksection noalias
		noinline nosuspend opaque or orelse packed pub resume return struct
		suspend switch test threadlocal try union unreachable usingnamespace
		var volatile while`),

	"nim": words(`addr and as asm bind block break case cast concept const
		continue converter defer discard distinct div do elif else end enum
		except export finally for from func if import in include interface is
		isnot iterator let macro method mixin mod nil not notin object of or
		out proc ptr raise ref return shl shr static template try tuple type
		using var when while xor yield`),

	"julia": words(`baremodule begin break catch const continue do else elseif
		end export false finally for function global if import let local macro
		module mutable primitive quote return struct true try using where
		while`),

	"c": words(`auto break case char const continue default do double else enum
		extern float for goto if inline int long register restrict return short
		signed sizeof static struct switch typedef union unsigned void volatile
		while bool true false`),

	"cpp": words(`alignas alignof and and_eq asm auto bitand bitor bool break
		case catch char class compl concept const consteval constexpr constinit
		const_cast continue decltype default delete do double dynamic_cast else
		enum explicit export extern false float for friend goto if inline int
		long mutable namespace new noexcept not not_eq nullptr operator or
		or_eq private protected public register reinterpret_cast requires
		return short signed sizeof static static_assert static_cast struct
		switch template this thread_local throw true try typedef typeid
		typename union unsigned using virtual void volatile wchar_t while xor
		xor_eq`),

	"haskell": words(`case class data default deriving do else foreign if
		import in infix infixl infixr instance let module newtype of then type
		where`),

	"ocaml": words(`and as assert asr begin class constraint do done downto
		else end exception external false for fun function functor if in
		include inherit initializer land lazy let lor lsl lsr lxor match method
		mod module mutable new nonrec object of open or private rec sig struct
		then to true try type val virtual when while with`),

	"awk": words(`BEGIN END break continue delete do else exit for function
		func getline if in next nextfile print printf return while`),

	"bash": words(`if then else elif fi case esac for while until do done in
		function select time coproc`),

	"crystal": words(`abstract alias as asm begin break case class def do else
		elsif end ensure enum extend false for fun if include lib macro module
		next nil of out private protected require rescue return select self
		sizeof struct super then true type typeof union uninitialized unless
		until when while with yield`),

	"d": words(`abstract alias align asm assert auto body bool break byte case
		cast catch cdouble cent cfloat char class const continue creal dchar
		debug default delegate delete deprecated do double else enum export
		extern false final finally float for foreach foreach_reverse function
		goto idouble if ifloat immutable import in inout int interface
		invariant ireal is lazy long macro mixin module new nothrow null out
		override package pragma private protected public pure real ref return
		scope shared short static struct super switch synchronized template
		this throw true try typeid typeof ubyte ucent uint ulong union unittest
		ushort version void wchar while with`),

	"fortran": words(`allocate allocatable call case character common contains
		continue cycle data deallocate dimension do else elseif end endif entry
		equivalence exit external function go goto if implicit integer intent
		interface intrinsic logical module namelist none nullify only open
		operator optional parameter pause pointer print private program public
		read real recursive result return save select sequence stop subroutine
		target then type use where while write`),

	"pascal": words(`and array asm begin case class const constructor
		destructor div do downto else end except exception file finally for
		function goto if implementation in inherited initialization inline
		interface label mod nil not object of on operator or out packed
		procedure program property raise record repeat resourcestring set shl
		shr string then to try type unit until uses var while with xor`),

	"perl": words(`my our local sub if elsif else unless while until for
		foreach do return last next redo package use require no and or not xor
		eq ne lt gt le ge cmp qw q qq tr y s m print printf`),

	"powershell": words(`begin break catch class continue data define do
		dynamicparam else elseif end enum exit filter finally for foreach from
		function hidden if in inlinescript param process return static switch
		throw trap try until using var while workflow`),

	// Tcl reserves nothing at the grammar level — every word is a command — so
	// a procedure named `set` would shadow the one the emitted program calls.
	// The list is the builtins these backends actually emit.
	"tcl": words(`if while for foreach proc set expr return break continue
		switch puts incr string list lindex llength lappend array upvar global
		uplevel catch error eval source namespace variable format open close
		gets exit`),

	"elixir": words(`after and catch case cond do else end fn for if import in
		nil not or quote raise receive require rescue true false unless unquote
		use when with`),

	"vala": words(`abstract as async base bool break case catch class const
		construct continue default delegate delete do double dynamic else enum
		errordomain extern false finally float for foreach get global if in
		inline int interface internal is lock long namespace new null out
		override owned params private protected public ref return set signal
		sizeof static string struct switch this throw throws true try typeof
		unowned ushort using value var virtual void volatile weak while yield`),

	"groovy": words(`abstract as assert boolean break byte case catch char
		class const continue def default do double else enum extends false
		final finally float for goto if implements import in instanceof int
		interface long native new null package private protected public return
		short static strictfp super switch synchronized this threadsafe throw
		throws trait transient true try void volatile while`),

	// A batch label or variable that is also a command word confuses cmd.exe
	// long before it confuses a parser.
	"bat": words(`if else for goto call set echo rem exit shift do in not exist
		errorlevel defined setlocal endlocal`),
}

// reservedLanguage maps a target to the language whose keywords it emits.
// Targets not listed use their own name; targets with no keyword table (the
// JSON and DSL emitters) are absent from reservedByLanguage and rename
// nothing.
var reservedLanguage = map[string]string{
	"javascript": "js",
	"chrome":     "js",
	"android":    "kotlin",
	"apk":        "kotlin",
	"ios":        "swift",
	"swift-pkg":  "swift",
}

// caseInsensitiveLanguages are the languages where `Label` and `label` are the
// same word, so a keyword check that respects case would miss half the
// collisions.
var caseInsensitiveLanguages = map[string]bool{
	"pascal": true, "fortran": true, "bat": true, "powershell": true,
}

// reservedFor returns the keyword set for a target, whether case matters when
// comparing against it, and whether the target has a table at all.
func reservedFor(target string) (set map[string]bool, foldCase bool, ok bool) {
	lang := targetAlias(target)
	if mapped, found := reservedLanguage[target]; found {
		lang = mapped
	} else if mapped, found := reservedLanguage[lang]; found {
		lang = mapped
	}
	set, ok = reservedByLanguage[lang]
	if !ok {
		return nil, false, false
	}
	return set, caseInsensitiveLanguages[lang], true
}
