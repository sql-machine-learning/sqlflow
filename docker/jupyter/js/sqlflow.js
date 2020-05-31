// Copyright 2020 The SQLFlow Authors. All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Support highlighting SQLFlow keywords in the SQLFlow magic cells in Jupyter Notebook.
// This file is based on https://github.com/codemirror/CodeMirror/blob/master/mode/sql/sql.js

// The file should be copied into the codemirror directory of Jupyter Notebook to take effect.
// (Typically `notebook/static/components/codemirror/mode/sqlflow/` in the Python site_packages directory)
// See https://github.com/sql-machine-learning/sqlflow/pull/1470/ for more details

// The anonymous function defines the codemirror mode for SQLFlow, based on the codemirror mode API
// (https://codemirror.net/doc/manual.html#modeapi)
// The token-related methods (tokenBase, tokenComment, tokenLiteral) are used by the `token` method to
// tokenize the input.
//
// The calls to `CodeMirror.defineMIME` associate the SQLFlow mode to MIME types (e.g. text/x-mysqlflow)
// to make it loadable by `custom.js`. This is also where we configure keywords.
(function(mod) {
    if (typeof exports == "object" && typeof module == "object") // CommonJS
        mod(require("../../lib/codemirror"));
    else if (typeof define == "function" && define.amd) // AMD
        define(["../../lib/codemirror"], mod);
    else // Plain browser env
        mod(CodeMirror);
})(function(CodeMirror) {
    "use strict";

    CodeMirror.defineMode("sqlflow", function(config, parserConfig) {
        "use strict";

        var client = parserConfig.client || {},
            atoms = parserConfig.atoms || {
                "false": true,
                "true": true,
                "null": true
            },
            builtin = parserConfig.builtin || {},
            keywords = parserConfig.keywords || {},
            operatorChars = parserConfig.operatorChars || /^[*+\-%<>!=&|~^]/,
            support = parserConfig.support || {},
            hooks = parserConfig.hooks || {},
            dateSQL = parserConfig.dateSQL || {
                "date": true,
                "time": true,
                "timestamp": true
            },
            backslashStringEscapes = parserConfig.backslashStringEscapes !== false,
            brackets = parserConfig.brackets || /^[\{}\(\)\[\]]/,
            punctuation = parserConfig.punctuation || /^[;.,:]/

        function tokenBase(stream, state) {
            var ch = stream.next();

            // call hooks from the mime type
            if (hooks[ch]) {
                var result = hooks[ch](stream, state);
                if (result !== false) return result;
            }

            if (support.hexNumber &&
                ((ch == "0" && stream.match(/^[xX][0-9a-fA-F]+/)) ||
                    (ch == "x" || ch == "X") && stream.match(/^'[0-9a-fA-F]+'/))) {
                // hex
                // ref: http://dev.mysql.com/doc/refman/5.5/en/hexadecimal-literals.html
                return "number";
            } else if (support.binaryNumber &&
                (((ch == "b" || ch == "B") && stream.match(/^'[01]+'/)) ||
                    (ch == "0" && stream.match(/^b[01]+/)))) {
                // bitstring
                // ref: http://dev.mysql.com/doc/refman/5.5/en/bit-field-literals.html
                return "number";
            } else if (ch.charCodeAt(0) > 47 && ch.charCodeAt(0) < 58) {
                // numbers
                // ref: http://dev.mysql.com/doc/refman/5.5/en/number-literals.html
                stream.match(/^[0-9]*(\.[0-9]+)?([eE][-+]?[0-9]+)?/);
                support.decimallessFloat && stream.match(/^\.(?!\.)/);
                return "number";
            } else if (ch == "?" && (stream.eatSpace() || stream.eol() || stream.eat(";"))) {
                // placeholders
                return "variable-3";
            } else if (ch == "'" || (ch == '"' && support.doubleQuote)) {
                // strings
                // ref: http://dev.mysql.com/doc/refman/5.5/en/string-literals.html
                state.tokenize = tokenLiteral(ch);
                return state.tokenize(stream, state);
            } else if ((((support.nCharCast && (ch == "n" || ch == "N")) ||
                        (support.charsetCast && ch == "_" && stream.match(/[a-z][a-z0-9]*/i))) &&
                    (stream.peek() == "'" || stream.peek() == '"'))) {
                // charset casting: _utf8'str', N'str', n'str'
                // ref: http://dev.mysql.com/doc/refman/5.5/en/string-literals.html
                return "keyword";
            } else if (support.commentSlashSlash && ch == "/" && stream.eat("/")) {
                // 1-line comment
                stream.skipToEnd();
                return "comment";
            } else if ((support.commentHash && ch == "#") ||
                (ch == "-" && stream.eat("-") && (!support.commentSpaceRequired || stream.eat(" ")))) {
                // 1-line comments
                // ref: https://kb.askmonty.org/en/comment-syntax/
                stream.skipToEnd();
                return "comment";
            } else if (ch == "/" && stream.eat("*")) {
                // multi-line comments
                // ref: https://kb.askmonty.org/en/comment-syntax/
                state.tokenize = tokenComment(1);
                return state.tokenize(stream, state);
            } else if (ch == ".") {
                // .1 for 0.1
                if (support.zerolessFloat && stream.match(/^(?:\d+(?:e[+-]?\d+)?)/i))
                    return "number";
                if (stream.match(/^\.+/))
                    return null
                // .table_name (ODBC)
                // // ref: http://dev.mysql.com/doc/refman/5.6/en/identifier-qualifiers.html
                if (support.ODBCdotTable && stream.match(/^[\w\d_]+/))
                    return "variable-2";
            } else if (operatorChars.test(ch)) {
                // operators
                stream.eatWhile(operatorChars);
                return "operator";
            } else if (brackets.test(ch)) {
                // brackets
                stream.eatWhile(brackets);
                return "bracket";
            } else if (punctuation.test(ch)) {
                // punctuation
                stream.eatWhile(punctuation);
                return "punctuation";
            } else if (ch == '{' &&
                (stream.match(/^( )*(d|D|t|T|ts|TS)( )*'[^']*'( )*}/) || stream.match(/^( )*(d|D|t|T|ts|TS)( )*"[^"]*"( )*}/))) {
                // dates (weird ODBC syntax)
                // ref: http://dev.mysql.com/doc/refman/5.5/en/date-and-time-literals.html
                return "number";
            } else {
                stream.eatWhile(/^[_\w\d]/);
                var word = stream.current().toLowerCase();
                // dates (standard SQL syntax)
                // ref: http://dev.mysql.com/doc/refman/5.5/en/date-and-time-literals.html
                if (dateSQL.hasOwnProperty(word) && (stream.match(/^( )+'[^']*'/) || stream.match(/^( )+"[^"]*"/)))
                    return "number";
                if (atoms.hasOwnProperty(word)) return "atom";
                if (builtin.hasOwnProperty(word)) return "builtin";
                if (keywords.hasOwnProperty(word)) return "keyword";
                if (client.hasOwnProperty(word)) return "string-2";
                return null;
            }
        }

        // 'string', with char specified in quote escaped by '\'
        function tokenLiteral(quote) {
            return function(stream, state) {
                var escaped = false,
                    ch;
                while ((ch = stream.next()) != null) {
                    if (ch == quote && !escaped) {
                        state.tokenize = tokenBase;
                        break;
                    }
                    escaped = backslashStringEscapes && !escaped && ch == "\\";
                }
                return "string";
            };
        }

        function tokenComment(depth) {
            return function(stream, state) {
                var m = stream.match(/^.*?(\/\*|\*\/)/)
                if (!m) stream.skipToEnd()
                else if (m[1] == "/*") state.tokenize = tokenComment(depth + 1)
                else if (depth > 1) state.tokenize = tokenComment(depth - 1)
                else state.tokenize = tokenBase
                return "comment"
            }
        }

        function pushContext(stream, state, type) {
            state.context = {
                prev: state.context,
                indent: stream.indentation(),
                col: stream.column(),
                type: type
            };
        }

        function popContext(state) {
            state.indent = state.context.indent;
            state.context = state.context.prev;
        }

        return {
            startState: function() {
                return {
                    tokenize: tokenBase,
                    context: null
                };
            },

            token: function(stream, state) {
                if (stream.sol()) {
                    if (state.context && state.context.align == null)
                        state.context.align = false;
                }
                if (state.tokenize == tokenBase && stream.eatSpace()) return null;

                var style = state.tokenize(stream, state);
                if (style == "comment") return style;

                if (state.context && state.context.align == null)
                    state.context.align = true;

                var tok = stream.current();
                if (tok == "(")
                    pushContext(stream, state, ")");
                else if (tok == "[")
                    pushContext(stream, state, "]");
                else if (state.context && state.context.type == tok)
                    popContext(state);
                return style;
            },

            indent: function(state, textAfter) {
                var cx = state.context;
                if (!cx) return CodeMirror.Pass;
                var closing = textAfter.charAt(0) == cx.type;
                if (cx.align) return cx.col + (closing ? 0 : 1);
                else return cx.indent + (closing ? 0 : config.indentUnit);
            },

            blockCommentStart: "/*",
            blockCommentEnd: "*/",
            lineComment: support.commentSlashSlash ? "//" : support.commentHash ? "#" : "--",
            closeBrackets: "()[]{}''\"\"``"
        };
    });

    (function() {
        "use strict";

        // `identifier`
        function hookIdentifier(stream) {
            // MySQL/MariaDB identifiers
            // ref: http://dev.mysql.com/doc/refman/5.6/en/identifier-qualifiers.html
            var ch;
            while ((ch = stream.next()) != null) {
                if (ch == "`" && !stream.eat("`")) return "variable-2";
            }
            stream.backUp(stream.current().length - 1);
            return stream.eatWhile(/\w/) ? "variable-2" : null;
        }

        // "identifier"
        function hookIdentifierDoublequote(stream) {
            // Standard SQL /SQLite identifiers
            // ref: http://web.archive.org/web/20160813185132/http://savage.net.au/SQL/sql-99.bnf.html#delimited%20identifier
            // ref: http://sqlite.org/lang_keywords.html
            var ch;
            while ((ch = stream.next()) != null) {
                if (ch == "\"" && !stream.eat("\"")) return "variable-2";
            }
            stream.backUp(stream.current().length - 1);
            return stream.eatWhile(/\w/) ? "variable-2" : null;
        }

        // variable token
        function hookVar(stream) {
            // variables
            // @@prefix.varName @varName
            // varName can be quoted with ` or ' or "
            // ref: http://dev.mysql.com/doc/refman/5.5/en/user-variables.html
            if (stream.eat("@")) {
                stream.match(/^session\./);
                stream.match(/^local\./);
                stream.match(/^global\./);
            }

            if (stream.eat("'")) {
                stream.match(/^.*'/);
                return "variable-2";
            } else if (stream.eat('"')) {
                stream.match(/^.*"/);
                return "variable-2";
            } else if (stream.eat("`")) {
                stream.match(/^.*`/);
                return "variable-2";
            } else if (stream.match(/^[0-9a-zA-Z$\.\_]+/)) {
                return "variable-2";
            }
            return null;
        };

        // short client keyword token
        function hookClient(stream) {
            // \N means NULL
            // ref: http://dev.mysql.com/doc/refman/5.5/en/null-values.html
            if (stream.eat("N")) {
                return "atom";
            }
            // \g, etc
            // ref: http://dev.mysql.com/doc/refman/5.5/en/mysql-commands.html
            return stream.match(/^[a-zA-Z.#!?]/) ? "variable-2" : null;
        }

        // these keywords are used by all SQL dialects (however, a mode can still overwrite it)
        var sqlKeywords = "alter and as asc between by count create delete desc distinct drop from group having in insert into is join like not on or order select set table union update values where limit ";
        var sqlflowKeywords = "to train predict analyze model label ";

        // turn a space-separated list into an array
        function set(str) {
            var obj = {},
                words = str.split(" ");
            for (var i = 0; i < words.length; ++i) obj[words[i]] = true;
            return obj;
        }

        CodeMirror.defineMIME("text/x-mysqlflow", {
            name: "sqlflow",
            client: set("charset clear connect edit ego exit go help nopager notee nowarning pager print prompt quit rehash source status system tee"),
            keywords: set(sqlKeywords + "accessible action add after algorithm all asensitive at authors auto_increment autocommit avg avg_row_length before binary binlog both btree cache call cascade cascaded case catalog_name chain change changed character check checkpoint checksum class_origin client_statistics close coalesce code collate collation collations column columns comment commit committed completion concurrent condition connection consistent constraint contains continue contributors convert cross current current_date current_time current_timestamp current_user cursor data database databases day_hour day_microsecond day_minute day_second deallocate dec declare default delay_key_write delayed delimiter des_key_file describe deterministic dev_pop dev_samp deviance diagnostics directory disable discard distinctrow div dual dumpfile each elseif enable enclosed end ends engine engines enum errors escape escaped even event events every execute exists exit explain extended fast fetch field fields first flush for force foreign found_rows full fulltext function general get global grant grants group group_concat handler hash help high_priority hosts hour_microsecond hour_minute hour_second if ignore ignore_server_ids import index index_statistics infile inner innodb inout insensitive insert_method install interval invoker isolation iterate key keys kill language last leading leave left level linear lines list load local localtime localtimestamp lock logs low_priority master master_heartbeat_period master_ssl_verify_server_cert masters match max max_rows maxvalue message_text middleint migrate min min_rows minute_microsecond minute_second mod mode modifies modify mutex mysql_errno natural next no no_write_to_binlog offline offset one online open optimize option optionally out outer outfile pack_keys parser partition partitions password phase plugin plugins prepare preserve prev primary privileges procedure processlist profile profiles purge query quick range read read_write reads real rebuild recover references regexp relaylog release remove rename reorganize repair repeatable replace require resignal restrict resume return returns revoke right rlike rollback rollup row row_format rtree savepoint schedule schema schema_name schemas second_microsecond security sensitive separator serializable server session share show signal slave slow smallint snapshot soname spatial specific sql sql_big_result sql_buffer_result sql_cache sql_calc_found_rows sql_no_cache sql_small_result sqlexception sqlstate sqlwarning ssl start starting starts status std stddev stddev_pop stddev_samp storage straight_join subclass_origin sum suspend table_name table_statistics tables tablespace temporary terminated trailing transaction trigger triggers truncate uncommitted undo uninstall unique unlock upgrade usage use use_frm user user_resources user_statistics using utc_date utc_time utc_timestamp value variables varying view views warnings when while with work write xa xor year_month zerofill begin do then else loop repeat"),
            builtin: set(sqlflowKeywords + "bool boolean bit blob decimal double float long longblob longtext medium mediumblob mediumint mediumtext time timestamp tinyblob tinyint tinytext text bigint int int1 int2 int3 int4 int8 integer float float4 float8 double char varbinary varchar varcharacter precision date datetime year unsigned signed numeric"),
            atoms: set("false true null unknown"),
            operatorChars: /^[*+\-%<>!=&|^]/,
            dateSQL: set("date time timestamp"),
            support: set("ODBCdotTable decimallessFloat zerolessFloat binaryNumber hexNumber doubleQuote nCharCast charsetCast commentHash commentSpaceRequired"),
            hooks: {
                "@": hookVar,
                "`": hookIdentifier,
                "\\": hookClient
            }
        });

        // Created to support specific hive keywords
        CodeMirror.defineMIME("text/x-hiveqlflow", {
            name: "sqlflow",
            keywords: set(sqlKeywords + "$elem$ $key$ $value$ add after all and archive as asc before between binary both bucket buckets by cascade case cast change cluster clustered clusterstatus collection column columns comment compute concatenate continue create cross cursor data database databases dbproperties deferred delete delimited desc describe directory disable distinct distribute drop else enable end escaped exclusive exists explain export extended external false fetch fields fileformat first format formatted from full function functions grant group having hold_ddltime idxproperties if import in index indexes inpath inputdriver inputformat insert intersect into is items join keys lateral left like lines load local location lock locks mapjoin materialized minus msck no_drop nocompress not of offline on option or order out outer outputdriver outputformat overwrite partition partitioned partitions percent plus preserve procedure purge range rcfile read readonly reads rebuild recordreader recordwriter recover reduce regexp rename repair replace restrict revoke right rlike row schema schemas semi sequencefile serde serdeproperties set shared show show_database sort sorted ssl statistics stored streamtable table tables tablesample tblproperties temporary terminated textfile then tmp touch transform trigger true unarchive undo union uniquejoin unlock update use using utc utc_tmestamp view when where while with"),
            builtin: set(sqlflowKeywords + "bool boolean long timestamp tinyint smallint bigint int float double date datetime unsigned string array struct map uniontype"),
            atoms: set("false true null unknown"),
            operatorChars: /^[*+\-%<>!=]/,
            dateSQL: set("date timestamp"),
            support: set("ODBCdotTable doubleQuote binaryNumber hexNumber")
        });
    }());
});