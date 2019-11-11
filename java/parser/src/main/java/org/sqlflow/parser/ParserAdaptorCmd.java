package org.sqlflow.parser;

import java.io.File;
import java.io.FileWriter;
import java.io.IOException;
import org.apache.commons.cli.CommandLine;
import org.apache.commons.cli.CommandLineParser;
import org.apache.commons.cli.DefaultParser;
import org.apache.commons.cli.HelpFormatter;
import org.apache.commons.cli.Options;
import org.apache.commons.cli.ParseException;
import org.apache.commons.io.FileUtils;

public class ParserAdaptorCmd {
  public static void main(String[] args) {
    Options options = new Options();
    options.addRequiredOption("p", "parse", true, "parser type, one of calcite|hiveql");
    options.addRequiredOption("i", "input_file", true, "input SQL file");
    options.addRequiredOption("o", "output_file", true, "output parsed result file");

    CommandLine line = null;
    try {
      CommandLineParser parser = new DefaultParser();
      line = parser.parse(options, args);
    } catch (ParseException e) {
      HelpFormatter formatter = new HelpFormatter();
      formatter.printHelp("Parser Command Line", options);
      System.exit(-1);
    }

    String parser_type = line.getOptionValue("p");
    String input_file = line.getOptionValue("i");
    String output_file = line.getOptionValue("o");

    if (!parser_type.equals("calcite") && !parser_type.equals("hiveql")) {
      System.err.printf("invalid parser type %s", parser_type);
      System.exit(-1);
    }

    String content = null;
    try {
      content = new String(FileUtils.readFileToByteArray(new File(input_file)));
    } catch (IOException e) {
      e.printStackTrace();
      System.exit(-1);
    }

    String jsonString;
    if (parser_type.equals("calcite")) {
      CalciteParserAdaptor parser = new CalciteParserAdaptor();
      jsonString = parser.ParseAndSplit(content).toJSONString();
    } else {
      HiveQLParserAdaptor parser = new HiveQLParserAdaptor();
      jsonString = parser.ParseAndSplit(content).toJSONString();
    }

    try {
      FileWriter file = new FileWriter(output_file);
      file.write(jsonString);
      file.flush();
    } catch (IOException e) {
      e.printStackTrace();
      System.exit(-1);
    }
  }
}
