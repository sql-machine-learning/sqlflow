package org.sqlflow;

import org.antlr.runtime.RecognitionException;
import org.antlr.runtime.Token;
import org.apache.hadoop.hive.ql.lib.Node;
import org.apache.hadoop.hive.ql.parse.ASTNode;
import org.apache.hadoop.hive.ql.parse.ParseDriver;
import org.apache.hadoop.hive.ql.parse.ParseError;
import org.apache.hadoop.hive.ql.parse.ParseException;

import java.io.IOException;
import java.lang.reflect.Field;
import java.lang.reflect.Type;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Paths;
import java.util.ArrayList;

/**
 * HiveQL Parser
 * Input: HiveQL File
 * Output:
 *  | success -> AST JSON
 *  | fail    -> [line, column]: error information
 */
public class HiveQLParser
{
    public static void main( String[] args )
            throws IOException, NoSuchFieldException, IllegalAccessException, ParseException {

        String file = args[0];
        String content = new String(Files.readAllBytes(Paths.get(file)), StandardCharsets.UTF_8);

        ParseDriver pd = new ParseDriver();

        try {
            ASTNode node = pd.parse(content);
            System.out.println(node);
        } catch (ParseException e) {
            Field errorsField = ParseException.class.getDeclaredField("errors");
            errorsField.setAccessible(true);
            ArrayList<ParseError> errors = (ArrayList<ParseError>) errorsField.get(e);

            Field reField = ParseError.class.getDeclaredField("re");
            reField.setAccessible(true);
            RecognitionException re = (RecognitionException) reField.get(errors.get(0));

            String errorMsg = "[" + re.line + "," + re.charPositionInLine + "]: " + e.getMessage();

            System.out.println(errorMsg);

            throw e;
        }
    }


}
